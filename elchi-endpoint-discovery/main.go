package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/CloudNativeWorks/elchi-plugins/pkg/config"
	elchiContext "github.com/CloudNativeWorks/elchi-plugins/pkg/context"
	"github.com/CloudNativeWorks/elchi-plugins/pkg/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}

	loggerCfg := &logger.Config{
		Level:  cfg.Log.Level,
		Format: cfg.Log.Format,
		Output: cfg.Log.Output,
	}
	log := logger.New(loggerCfg)

	ctx := elchiContext.WithConfig(context.Background(), cfg)

	// Get discovery interval from environment variable
	intervalStr := os.Getenv("DISCOVERY_INTERVAL")
	if intervalStr == "" {
		intervalStr = "30" // default 30 seconds
	}

	intervalSec, err := strconv.Atoi(intervalStr)
	if err != nil {
		log.WithError(err).Warn("Invalid DISCOVERY_INTERVAL, using default 30 seconds")
		intervalSec = 30
	}

	interval := time.Duration(intervalSec) * time.Second

	log.WithPlugin("endpoint-discovery").Info("Starting elchi-endpoint-discovery service")
	log.WithFields(map[string]interface{}{
		"token_configured":   cfg.Elchi.Token != "",
		"discovery_interval": interval.String(),
	}).Info("Configuration loaded")

	clientset, err := getKubernetesClient(cfg.KubeConfig)
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kubernetes client")
		return
	}

	// Continuous discovery loop
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run discovery immediately on startup
	runDiscovery(ctx, log, clientset)

	// Then run on schedule
	for {
		select {
		case <-ticker.C:
			runDiscovery(ctx, log, clientset)
		case <-ctx.Done():
			log.Info("Shutdown signal received, stopping discovery")
			return
		}
	}
}

type ClusterInfo struct {
	Name    string `json:"cluster_name"`
	Version string `json:"cluster_version"`
}

type NodeInfo struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Addresses map[string]string `json:"addresses"`
}

type DiscoveryResult struct {
	Timestamp   time.Time   `json:"timestamp"`
	ClusterInfo ClusterInfo `json:"cluster_info"`
	NodeCount   int         `json:"node_count"`
	Nodes       []NodeInfo  `json:"nodes"`
	Duration    string      `json:"discovery_duration"`
}

func runDiscovery(ctx context.Context, log *logger.Logger, clientset *kubernetes.Clientset) {
	discoveryStart := time.Now()

	// Get cluster info
	clusterInfo := getClusterInfo(ctx, clientset)

	nodes, err := discoverNodes(ctx, clientset)
	if err != nil {
		log.WithError(err).Error("Failed to discover nodes")
		return
	}

	// Build discovery result
	result := DiscoveryResult{
		Timestamp:   time.Now(),
		ClusterInfo: clusterInfo,
		NodeCount:   len(nodes.Items),
		Nodes:       make([]NodeInfo, 0, len(nodes.Items)),
		Duration:    time.Since(discoveryStart).String(),
	}

	for _, node := range nodes.Items {
		nodeInfo := NodeInfo{
			Name:      node.Name,
			Status:    getNodeStatus(&node),
			Version:   node.Status.NodeInfo.KubeletVersion,
			Addresses: make(map[string]string),
		}

		for _, address := range node.Status.Addresses {
			nodeInfo.Addresses[string(address.Type)] = address.Address
		}

		result.Nodes = append(result.Nodes, nodeInfo)
	}

	// Print as pretty JSON
	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal discovery result to JSON")
		return
	}

	fmt.Println(string(jsonOutput))

	log.WithFields(map[string]interface{}{
		"node_count":      result.NodeCount,
		"duration":        result.Duration,
		"cluster_name":    result.ClusterInfo.Name,
		"cluster_version": result.ClusterInfo.Version,
	}).Info("Discovery completed")
}

func getClusterInfo(ctx context.Context, clientset *kubernetes.Clientset) ClusterInfo {
	info := ClusterInfo{
		Name:    "unknown",
		Version: "unknown",
	}

	// Try to get cluster name from kubeadm configmap
	configMap, err := clientset.CoreV1().ConfigMaps("kube-system").Get(ctx, "kubeadm-config", metav1.GetOptions{})
	if err == nil && configMap != nil {
		if clusterConfig, ok := configMap.Data["ClusterConfiguration"]; ok {
			// Simple parsing - in production you'd use proper YAML parsing
			if len(clusterConfig) > 0 {
				info.Name = "kubernetes-cluster"
			}
		}
	}

	// Get cluster version from server version
	version, err := clientset.Discovery().ServerVersion()
	if err == nil && version != nil {
		info.Version = version.GitVersion
	}

	// Try to get cluster name from nodes if not found
	if info.Name == "unknown" {
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil && len(nodes.Items) > 0 {
			// Use first node's cluster name label if exists
			if clusterName, ok := nodes.Items[0].Labels["kubernetes.io/cluster-name"]; ok {
				info.Name = clusterName
			} else if nodes.Items[0].Spec.ProviderID != "" {
				info.Name = "kubernetes-cluster"
			}
		}
	}

	return info
}

func getKubernetesClient(kubeConfigPath string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	if kubeConfigPath == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			if home := homedir.HomeDir(); home != "" {
				kubeConfigPath = filepath.Join(home, ".kube", "config")
			}
		}
	}

	if config == nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

func discoverNodes(ctx context.Context, clientset *kubernetes.Clientset) (*v1.NodeList, error) {
	return clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

func getNodeStatus(node *v1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			if condition.Status == v1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}
