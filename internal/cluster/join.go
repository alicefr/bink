package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bootc-dev/bink/internal/ssh"
)

// JoinOptions holds options for joining a node to the cluster
type JoinOptions struct {
	NodeName     string
	ControlPlane string
	Timeout      time.Duration
}

// Join joins a worker node to the cluster
func (c *Cluster) Join(ctx context.Context, opts JoinOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Minute
	}

	if opts.ControlPlane == "" {
		opts.ControlPlane = c.controlPlane
	}

	nodeName := opts.NodeName
	controlPlane := opts.ControlPlane

	c.logger.Info("")
	c.logger.Infof("=== Generating join command from %s ===", controlPlane)

	// Create SSH client for control plane
	cpSSHClient := ssh.NewClientForNode(controlPlane, c.logger)

	// Generate join command
	joinCommand, err := c.generateJoinCommand(ctx, cpSSHClient)
	if err != nil {
		return fmt.Errorf("failed to generate join command: %w", err)
	}

	c.logger.Infof("Join command: %s", joinCommand)

	c.logger.Info("")
	c.logger.Infof("=== Waiting for %s to be ready ===", nodeName)

	// Wait for cloud-init on the new node
	if err := c.WaitForCloudInit(ctx, nodeName, opts.Timeout); err != nil {
		return err
	}

	c.logger.Info("")
	c.logger.Infof("=== Joining %s to the cluster ===", nodeName)

	// Create SSH client for the new node
	nodeSSHClient := ssh.NewClientForNode(nodeName, c.logger)

	// Execute join command
	if err := nodeSSHClient.ExecWithOutput(ctx, fmt.Sprintf("sudo %s", joinCommand)); err != nil {
		return fmt.Errorf("failed to join node: %w", err)
	}

	c.logger.Info("")
	c.logger.Infof("✅ Node %s successfully joined the cluster!", nodeName)
	c.logger.Info("")
	c.logger.Info("Verify with:")
	c.logger.Infof("  bink node ssh %s", controlPlane)
	c.logger.Info("  kubectl get nodes")

	return nil
}

// generateJoinCommand generates a fresh join command from the control plane
func (c *Cluster) generateJoinCommand(ctx context.Context, cpSSHClient *ssh.Client) (string, error) {
	output, err := cpSSHClient.Exec(ctx, "sudo kubeadm token create --print-join-command")
	if err != nil {
		return "", fmt.Errorf("failed to generate join command: %w", err)
	}

	// Trim whitespace
	joinCommand := strings.TrimSpace(output)

	if joinCommand == "" {
		return "", fmt.Errorf("join command is empty")
	}

	return joinCommand, nil
}
