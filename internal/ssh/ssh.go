package ssh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// Client provides SSH operations for connecting to VMs via containers
type Client struct {
	containerName string
	host          string
	port          string
	keyPath       string
	user          string
	logger        *logrus.Logger
}

// Config holds SSH client configuration
type Config struct {
	ContainerName string // Podman container name
	Host          string // SSH host (usually "localhost" for port-forwarded VMs)
	Port          string // SSH port (usually "2222" for port-forwarded VMs)
	KeyPath       string // Path to SSH private key
	User          string // SSH user (usually "core")
	Logger        *logrus.Logger
}

// NewClient creates a new SSH client
func NewClient(cfg Config) *Client {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}
	return &Client{
		containerName: cfg.ContainerName,
		host:          cfg.Host,
		port:          cfg.Port,
		keyPath:       cfg.KeyPath,
		user:          cfg.User,
		logger:        cfg.Logger,
	}
}

// Exec executes a command via SSH and returns stdout
func (c *Client) Exec(ctx context.Context, command string) (string, error) {
	sshArgs := c.buildSSHArgs(command)

	cmd := exec.CommandContext(ctx, "podman", append([]string{"exec", c.containerName, "ssh"}, sshArgs...)...)

	c.logger.Debugf("Running: podman exec %s ssh %s", c.containerName, strings.Join(sshArgs, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh exec failed: %w: %s", err, string(output))
	}

	return string(output), nil
}

// ExecWithOutput executes a command via SSH, streaming output to stdout/stderr
func (c *Client) ExecWithOutput(ctx context.Context, command string) error {
	sshArgs := c.buildSSHArgs(command)

	cmd := exec.CommandContext(ctx, "podman", append([]string{"exec", c.containerName, "ssh"}, sshArgs...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	c.logger.Debugf("Running: podman exec %s ssh %s", c.containerName, strings.Join(sshArgs, " "))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh exec failed: %w", err)
	}

	return nil
}

// Interactive starts an interactive SSH session
func (c *Client) Interactive(ctx context.Context) error {
	sshArgs := c.buildSSHArgs("")

	cmd := exec.CommandContext(ctx, "podman", append([]string{"exec", "-ti", c.containerName, "ssh"}, sshArgs...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	c.logger.Infof("Connecting to %s (SSH: %s:%s, cluster IP) as user %s",
		c.containerName, c.host, c.port, c.user)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interactive ssh failed: %w", err)
	}

	return nil
}

// CopyTo copies a file to the remote host via SCP
func (c *Client) CopyTo(ctx context.Context, localPath, remotePath string) error {
	scpArgs := []string{
		"-P", c.port,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", c.keyPath,
		localPath,
		fmt.Sprintf("%s@%s:%s", c.user, c.host, remotePath),
	}

	cmd := exec.CommandContext(ctx, "podman", append([]string{"exec", c.containerName, "scp"}, scpArgs...)...)

	c.logger.Debugf("Running: podman exec %s scp %s", c.containerName, strings.Join(scpArgs, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("scp failed: %w: %s", err, string(output))
	}

	return nil
}

// WaitForSSH waits for SSH to become available
func (c *Client) WaitForSSH(ctx context.Context, maxRetries int) error {
	c.logger.Infof("Waiting for SSH to be ready on %s...", c.host)

	for i := 1; i <= maxRetries; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		sshArgs := c.buildSSHArgs("true")
		sshArgs = append(sshArgs, "-o", "ConnectTimeout=2")

		cmd := exec.CommandContext(ctx, "podman", append([]string{"exec", c.containerName, "ssh"}, sshArgs...)...)

		if err := cmd.Run(); err == nil {
			c.logger.Info("✓ SSH is ready")
			return nil
		}

		if i == maxRetries {
			return fmt.Errorf("timeout waiting for SSH to be ready after %d attempts", maxRetries)
		}

		c.logger.Debug(".")
	}

	return nil
}

// buildSSHArgs constructs the SSH command arguments
func (c *Client) buildSSHArgs(command string) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", c.keyPath,
		"-p", c.port,
		fmt.Sprintf("%s@%s", c.user, c.host),
	}

	if command != "" {
		args = append(args, command)
	}

	return args
}
