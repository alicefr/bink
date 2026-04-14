package network

import (
	"context"
	"fmt"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/bootc-dev/bink/internal/podman"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	podman *podman.Client
}

func NewManager() *Manager {
	return &Manager{
		podman: podman.NewClient(),
	}
}

func (m *Manager) Create(ctx context.Context, name, subnet string) error {
	exists, err := m.podman.NetworkExists(ctx, name)
	if err != nil {
		return fmt.Errorf("checking if network exists: %w", err)
	}

	if exists {
		logrus.Infof("Network '%s' already exists", name)
		return nil
	}

	return m.podman.NetworkCreate(ctx, name, subnet)
}

func (m *Manager) GetSubnet(ctx context.Context, name string) (string, error) {
	subnet, err := m.podman.NetworkInspect(ctx, name, "{{range .Subnets}}{{.Subnet}}{{end}}")
	if err != nil {
		return "", fmt.Errorf("inspecting network: %w", err)
	}
	return subnet, nil
}

func (m *Manager) EnsureClusterNetwork(ctx context.Context) error {
	logrus.Info("Ensuring cluster network exists")

	if err := m.Create(ctx, config.DefaultNetworkName, config.DefaultSubnet); err != nil {
		return fmt.Errorf("creating cluster network: %w", err)
	}

	subnet, err := m.GetSubnet(ctx, config.DefaultNetworkName)
	if err != nil {
		return fmt.Errorf("getting network subnet: %w", err)
	}

	logrus.Infof("Cluster network '%s' ready with subnet %s", config.DefaultNetworkName, subnet)
	return nil
}
