package main

import (
	"context"
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
		"token_configured":    cfg.Elchi.Token != "",
		"discovery_interval":  interval.String(),
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

func runDiscovery(ctx context.Context, log *logger.Logger, clientset *kubernetes.Clientset) {
	discoveryStart := time.Now()
	
	nodes, err := discoverNodes(ctx, clientset)
	if err != nil {
		log.WithError(err).Error("Failed to discover nodes")
		return
	}

	log.WithFields(map[string]interface{}{
		"node_count": len(nodes.Items),
		"duration":   time.Since(discoveryStart).String(),
	}).Info("Node discovery completed")

	for _, node := range nodes.Items {
		printNodeInfo(ctx, log, &node)
	}
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

func printNodeInfo(ctx context.Context, log *logger.Logger, node *v1.Node) {
	cfg := elchiContext.GetConfig(ctx)
	
	nodeLogger := log.WithFields(map[string]interface{}{
		"node_name":    node.Name,
		"node_status":  getNodeStatus(node),
		"node_version": node.Status.NodeInfo.KubeletVersion,
		"component":    "node-discovery",
	})

	addresses := make(map[string]string)
	for _, address := range node.Status.Addresses {
		addresses[string(address.Type)] = address.Address
	}
	
	nodeLogger = nodeLogger.WithField("addresses", addresses)
	
	if cfg != nil && cfg.Elchi.Token != "" {
		nodeLogger = nodeLogger.WithField("token_available", true)
	}
	
	nodeLogger.Info("Node discovered")
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