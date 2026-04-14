package node

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bootc-dev/bink/internal/cluster"
	"github.com/bootc-dev/bink/internal/dns"
	"github.com/bootc-dev/bink/internal/node"
)

func newAddCmd() *cobra.Command {
	var controlPlane string
	var imagesImage string

	cmd := &cobra.Command{
		Use:   "add <node-name>",
		Short: "Add a worker node to the cluster",
		Long:  "Create a new worker node and join it to the Kubernetes cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logrus.New()
			return runAdd(cmd.Context(), args[0], controlPlane, imagesImage, logger)
		},
	}

	cmd.Flags().StringVarP(&controlPlane, "control-plane", "c", "node1", "Control plane node name")
	cmd.Flags().StringVar(&imagesImage, "images-image", "localhost/fedora-bootc-k8s-image:latest", "Container image containing base VM images")

	return cmd
}

func runAdd(ctx context.Context, nodeName, controlPlane, imagesImage string, logger *logrus.Logger) error {
	logger.Infof("=== Creating worker node %s ===", nodeName)
	logger.Info("")

	// Step 1: Create the new node
	logger.Info("Step 1: Creating worker node...")
	logger.Infof("VM images container: %s", imagesImage)

	workerNode := node.NewWithConfig(nodeName, false, node.Config{
		ImagesImage: imagesImage,
	})

	exists, err := workerNode.Exists(ctx)
	if err != nil {
		return fmt.Errorf("checking if node exists: %w", err)
	}

	if exists {
		return fmt.Errorf("node %s already exists", nodeName)
	}

	if err := workerNode.Create(ctx); err != nil {
		return fmt.Errorf("creating node: %w", err)
	}
	logger.Info("")

	// Step 2: Add DNS entry
	logger.Info("Step 2: Adding DNS entry...")
	dnsMgr := dns.NewManager(dns.Config{
		DNSServer: controlPlane,
		Logger:    logger,
	})

	if err := dnsMgr.AddEntry(ctx, nodeName); err != nil {
		return fmt.Errorf("adding DNS entry: %w", err)
	}
	logger.Info("")

	// Step 3: Join to cluster
	logger.Info("Step 3: Joining node to cluster...")
	clusterMgr := cluster.New(cluster.Config{
		Name:         "bink",
		ControlPlane: controlPlane,
		Logger:       logger,
	})

	if err := clusterMgr.Join(ctx, cluster.JoinOptions{
		NodeName:     nodeName,
		ControlPlane: controlPlane,
	}); err != nil {
		return fmt.Errorf("joining node to cluster: %w", err)
	}

	return nil
}
