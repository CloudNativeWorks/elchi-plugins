package discovery

import (
	"context"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Service struct {
	client      *kubernetes.Clientset
	clusterName string
}

func NewService(client *kubernetes.Clientset, clusterName string) *Service {
	return &Service{
		client:      client,
		clusterName: clusterName,
	}
}

func (s *Service) DiscoverNodes(ctx context.Context) (*DiscoveryResult, error) {
	discoveryStart := time.Now()

	// Get cluster info
	clusterInfo := s.getClusterInfo(ctx)

	// Get nodes
	nodes, err := s.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Build discovery result
	result := &DiscoveryResult{
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

	return result, nil
}

func (s *Service) getClusterInfo(ctx context.Context) ClusterInfo {
	info := ClusterInfo{
		Name:    "unknown",
		Version: "unknown",
	}

	// Priority 1: Use cluster name from config if provided
	if s.clusterName != "" {
		info.Name = s.clusterName
	} else {
		// Method 1: Try to get cluster name from kubeadm configmap
	if configMap, err := s.client.CoreV1().ConfigMaps("kube-system").Get(ctx, "kubeadm-config", metav1.GetOptions{}); err == nil {
		if clusterConfig, ok := configMap.Data["ClusterConfiguration"]; ok {
			// Look for clusterName in the YAML
			for _, line := range strings.Split(clusterConfig, "\n") {
				if strings.Contains(line, "clusterName:") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						name := strings.TrimSpace(parts[1])
						name = strings.Trim(name, "\"'")
						if name != "" {
							info.Name = name
							break
						}
					}
				}
			}
		}
	}

	// Method 2: Try cluster-info ConfigMap
	if info.Name == "unknown" {
		if configMap, err := s.client.CoreV1().ConfigMaps("kube-public").Get(ctx, "cluster-info", metav1.GetOptions{}); err == nil {
			// Check for cluster name in annotations or labels
			if name, ok := configMap.Labels["kubernetes.io/cluster-name"]; ok && name != "" {
				info.Name = name
			} else if name, ok := configMap.Annotations["cluster-name"]; ok && name != "" {
				info.Name = name
			}
		}
	}

	// Method 3: Try to get from kube-system namespace configmaps
	if info.Name == "unknown" {
		configMaps, err := s.client.CoreV1().ConfigMaps("kube-system").List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, cm := range configMaps.Items {
				// Check for cluster name in various configmaps
				if strings.Contains(cm.Name, "cluster") {
					if name, ok := cm.Data["cluster-name"]; ok && name != "" {
						info.Name = name
						break
					}
				}
			}
		}
	}

	// Method 4: Try to get from nodes
	if info.Name == "unknown" {
		nodes, err := s.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil && len(nodes.Items) > 0 {
			// Check various labels
			for _, label := range []string{
				"kubernetes.io/cluster-name",
				"cluster.x-k8s.io/cluster-name",
				"cluster-name",
			} {
				if name, ok := nodes.Items[0].Labels[label]; ok && name != "" {
					info.Name = name
					break
				}
			}
			
			// If still not found, try to extract from provider ID
			if info.Name == "unknown" && nodes.Items[0].Spec.ProviderID != "" {
				// Try to extract cluster name from provider ID
				// Format examples: aws:///us-west-2a/i-12345, gce://project/zone/instance
				providerID := nodes.Items[0].Spec.ProviderID
				if strings.Contains(providerID, "://") {
					info.Name = "kubernetes-cluster"
				}
			}
		}
	}

	// Method 5: Default based on context
	if info.Name == "unknown" {
		// Try to get from kube-system namespace name or generate default
		if ns, err := s.client.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{}); err == nil {
			if clusterName, ok := ns.Labels["cluster-name"]; ok && clusterName != "" {
				info.Name = clusterName
			} else {
				// Use a default name
				info.Name = "kubernetes-cluster"
			}
		}
	}
	}

	// Get cluster version from server version
	if version, err := s.client.Discovery().ServerVersion(); err == nil && version != nil {
		info.Version = version.GitVersion
	}

	return info
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