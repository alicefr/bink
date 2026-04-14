package ssh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

type TunnelConfig struct {
	ContainerName string
	Host          string
	Port          string
	KeyPath       string
	User          string
	LocalPort     string
	RemotePort    string
	BindAddress   string
	Logger        *logrus.Logger
}

func StartTunnel(ctx context.Context, cfg TunnelConfig) error {
	if cfg.Logger == nil {
		cfg.Logger = logrus.New()
	}

	bindAddr := cfg.BindAddress
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}

	sshArgs := []string{
		"-N",
		"-L", fmt.Sprintf("%s:%s:localhost:%s", bindAddr, cfg.LocalPort, cfg.RemotePort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=60",
		"-i", cfg.KeyPath,
		"-p", cfg.Port,
		fmt.Sprintf("%s@%s", cfg.User, cfg.Host),
	}

	podmanArgs := append([]string{"exec", "-d", cfg.ContainerName, "ssh"}, sshArgs...)

	cfg.Logger.Debugf("Starting SSH tunnel: podman %s", strings.Join(podmanArgs, " "))
	cfg.Logger.Infof("Starting SSH port forwarding: %s:%s -> VM:%s", bindAddr, cfg.LocalPort, cfg.RemotePort)

	cmd := exec.CommandContext(ctx, "podman", podmanArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("starting SSH tunnel: %w: %s", err, string(output))
	}

	return nil
}

func IsTunnelActive(ctx context.Context, containerName, port string) (bool, error) {
	cmd := exec.CommandContext(ctx, "podman", "exec", containerName, "ss", "-tln")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("checking tunnel status: %w: %s", err, string(output))
	}

	return strings.Contains(string(output), fmt.Sprintf(":%s", port)), nil
}
