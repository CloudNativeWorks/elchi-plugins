package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CloudNativeWorks/elchi-plugins/elchi-endpoint-discovery/api"
	"github.com/CloudNativeWorks/elchi-plugins/elchi-endpoint-discovery/discovery"
	"github.com/CloudNativeWorks/elchi-plugins/pkg/config"
	elchiContext "github.com/CloudNativeWorks/elchi-plugins/pkg/context"
	"github.com/CloudNativeWorks/elchi-plugins/pkg/logger"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	// Validate required config fields
	if cfg.ClusterName == "" {
		log.Fatal("Cluster name is required. Please set cluster_name in config or CLUSTER_NAME environment variable")
		return
	}

	// Get discovery interval from config
	intervalSec := cfg.DiscoveryInterval
	if intervalSec <= 0 {
		intervalSec = 30 // default 30 seconds if not set or invalid
	}

	interval := time.Duration(intervalSec) * time.Second

	log.WithPlugin("endpoint-discovery").Info("Starting elchi-endpoint-discovery service")
	log.WithFields(map[string]any{
		"token_configured":   cfg.Elchi.Token != "",
		"api_endpoint":       cfg.Elchi.APIEndpoint,
		"discovery_interval": interval.String(),
		"insecure_tls":       cfg.Elchi.InsecureSkipVerify,
	}).Info("Configuration loaded")

	// Create Kubernetes client
	clientset, err := getKubernetesClient()
	if err != nil {
		log.WithError(err).Fatal("Failed to create Kubernetes client")
		return
	}

	// Create discovery service
	discoveryService := discovery.NewService(clientset, cfg.ClusterName)

	// Create API client
	apiClient := api.NewClient(cfg, log)

	// Continuous discovery loop
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run discovery immediately on startup
	runDiscovery(ctx, log, discoveryService, apiClient)

	// Then run on schedule
	for {
		select {
		case <-ticker.C:
			runDiscovery(ctx, log, discoveryService, apiClient)
		case <-ctx.Done():
			log.Info("Shutdown signal received, stopping discovery")
			return
		}
	}
}

func runDiscovery(ctx context.Context, log *logger.Logger, discoveryService *discovery.Service, apiClient *api.Client) {
	// Perform discovery
	result, err := discoveryService.DiscoverNodes(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to discover nodes")
		return
	}

	// Print as pretty JSON to stdout
	jsonOutput, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.WithError(err).Error("Failed to marshal discovery result to JSON")
		return
	}

	fmt.Println(string(jsonOutput))

	// Send to API if configured
	if err := apiClient.SendDiscoveryResult(result); err != nil {
		log.WithError(err).Error("Failed to send discovery result to API")
		// Don't return here - we still want to continue discovery even if API fails
	}

	log.WithFields(map[string]any{
		"node_count":      result.NodeCount,
		"duration":        result.Duration,
		"cluster_name":    result.ClusterInfo.Name,
		"cluster_version": result.ClusterInfo.Version,
	}).Info("Discovery completed")
}

func getKubernetesClient() (*kubernetes.Clientset, error) {
	// This service ONLY runs inside Kubernetes
	// It discovers nodes of the cluster it's running in
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w. This service must run inside a Kubernetes cluster", err)
	}

	return kubernetes.NewForConfig(config)
}
