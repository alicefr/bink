package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/bootc-dev/bink/internal/podman"
)

func newStopCmd() *cobra.Command {
	var force bool
	var removeData bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the cluster",
		Long:  "Stop and remove all cluster nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logrus.New()
			return runStop(cmd.Context(), logger, force, removeData)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force stop containers")
	cmd.Flags().BoolVar(&removeData, "remove-data", false, "Remove node data (overlay disks, cloud-init ISOs, SSH keys)")

	return cmd
}

func runStop(ctx context.Context, logger *logrus.Logger, force, removeData bool) error {
	logger.Info("=== Stopping cluster ===")
	logger.Info("")

	podmanClient := podman.NewClient()

	// Find all cluster containers
	clusterName := viper.GetString("cluster.name")

	// Get all k8s-* containers first
	filter := fmt.Sprintf("name=%s", config.ContainerNamePrefix)
	allContainers, err := podmanClient.ContainerList(ctx, filter)
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	// Filter containers based on cluster name
	var containers []string
	if clusterName != "" && clusterName != config.DefaultNetworkName {
		// Named cluster: only include k8s-{cluster}-* containers
		prefix := fmt.Sprintf("%s%s-", config.ContainerNamePrefix, clusterName)
		for _, container := range allContainers {
			if container != "" && hasPrefix(container, prefix) {
				containers = append(containers, container)
			}
		}
		logger.Infof("Stopping cluster: %s", clusterName)
	} else {
		// Default cluster: only include k8s-* containers WITHOUT a cluster name
		// (i.e., containers that don't have a hyphen after the k8s- prefix before the node name)
		for _, container := range allContainers {
			if container != "" && isDefaultClusterContainer(container) {
				containers = append(containers, container)
			}
		}
		logger.Info("Stopping default cluster")
	}

	if len(containers) == 0 {
		logger.Info("No cluster nodes found")
		return nil
	}

	logger.Infof("Found %d node(s) to stop:", len(containers))
	for _, container := range containers {
		logger.Infof("  - %s", container)
	}
	logger.Info("")

	// Stop and remove each container
	for _, container := range containers {
		if container == "" {
			continue
		}

		logger.Infof("Stopping container: %s", container)
		if err := podmanClient.ContainerStop(ctx, container); err != nil {
			logger.Warnf("Failed to stop %s: %v", container, err)
		}

		logger.Infof("Removing container: %s", container)
		if err := podmanClient.ContainerRemove(ctx, container, force); err != nil {
			logger.Warnf("Failed to remove %s: %v", container, err)
		}
	}

	logger.Info("")
	logger.Info("✅ All cluster nodes stopped and removed")

	if removeData {
		logger.Info("")
		logger.Info("Removing cluster data...")

		if err := removeClusterData(logger, containers); err != nil {
			logger.Warnf("Failed to remove some data: %v", err)
			logger.Warn("You may need to manually clean up:")
			logger.Warn("  - Cluster keys volume: podman volume rm cluster-keys")
			logger.Warn("  - Kubeconfig: rm -f ./vm/kubeconfig")
		} else {
			logger.Info("✅ All cluster data removed")
		}
	}

	return nil
}

func removeClusterData(logger *logrus.Logger, containers []string) error {
	var errors []string

	podmanClient := podman.NewClient()
	ctx := context.Background()

	// Remove cluster-keys volume
	logger.Info("Removing cluster-keys volume...")
	if err := podmanClient.VolumeRemove(ctx, "cluster-keys"); err != nil {
		logger.Warnf("Failed to remove cluster-keys volume: %v", err)
		errors = append(errors, err.Error())
	} else {
		logger.Info("Removed cluster-keys volume")
	}

	// Remove kubeconfig
	kubeconfigPath := filepath.Join(config.DefaultKubeconfigDir, "kubeconfig")
	if err := os.Remove(kubeconfigPath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("Failed to remove kubeconfig %s: %v", kubeconfigPath, err)
		errors = append(errors, err.Error())
	} else if err == nil {
		logger.Infof("Removed kubeconfig: %s", kubeconfigPath)
	}

	logger.Info("Note: Overlay disks and cloud-init ISOs are stored in ephemeral container storage and removed automatically")

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d error(s) during cleanup", len(errors))
	}

	return nil
}

// hasPrefix checks if a string starts with the given prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// isDefaultClusterContainer checks if a container name belongs to the default cluster
// Default cluster containers have the format: k8s-<node> (e.g., k8s-node1)
// Named cluster containers have the format: k8s-<cluster>-<node> (e.g., k8s-mycluster-node1)
func isDefaultClusterContainer(name string) bool {
	if !hasPrefix(name, config.ContainerNamePrefix) {
		return false
	}

	// Remove the k8s- prefix
	remainder := name[len(config.ContainerNamePrefix):]

	// If the remainder contains a hyphen, it's a named cluster container
	// (e.g., "mycluster-node1" has a hyphen)
	// Default cluster containers have no hyphen (e.g., "node1")
	for i := 0; i < len(remainder); i++ {
		if remainder[i] == '-' {
			return false
		}
	}

	return true
}
