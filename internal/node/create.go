package node

import (
	"context"
	"fmt"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/bootc-dev/bink/internal/podman"
	"github.com/bootc-dev/bink/internal/virsh"
	"github.com/sirupsen/logrus"
)

func (n *Node) createContainer(ctx context.Context) error {
	exists, err := n.Exists(ctx)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("container %s already exists", n.ContainerName)
	}

	logrus.Infof("Creating container %s", n.ContainerName)
	logrus.Infof("Using images container: %s", n.ImagesImage)

	opts := &podman.ContainerCreateOptions{
		Name:    n.ContainerName,
		Image:   config.DefaultClusterImage,
		Network: config.DefaultNetworkName,
		Devices: []string{"/dev/kvm", "/dev/fuse"},
		Mounts: []string{
			fmt.Sprintf("type=image,source=%s,destination=/images", n.ImagesImage),
		},
		Volumes: []string{
			"cluster-keys:/var/run/cluster:z",
		},
	}

	if n.IsControlPlane {
		opts.Ports = []string{"6443:6443"}
	}

	containerID, err := n.podman.ContainerCreate(ctx, opts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	logrus.Infof("Container %s created: %s", n.ContainerName, containerID)

	containerIP, err := n.podman.ContainerInspect(ctx, n.ContainerName, "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}")
	if err != nil {
		return fmt.Errorf("getting container IP: %w", err)
	}

	logrus.Infof("Container IP: %s (VM will inherit this via passt)", containerIP)

	// Create workspace directory for overlay disks and cloud-init ISOs
	if err := n.podman.ContainerExecQuiet(ctx, n.ContainerName, []string{"mkdir", "-p", "/workspace"}); err != nil {
		return fmt.Errorf("creating workspace directory: %w", err)
	}

	return nil
}

func (n *Node) setupSSHKeys(ctx context.Context) error {
	logrus.Info("Setting up cluster SSH key")

	// Check if key already exists in the volume
	checkCmd := []string{"test", "-f", config.ClusterKeyPath}
	err := n.podman.ContainerExecQuiet(ctx, n.ContainerName, checkCmd)
	if err == nil {
		logrus.Info("Using existing cluster SSH key")
		return nil
	}

	logrus.Info("Generating cluster SSH key in cluster-keys volume")

	// Generate SSH key inside the container
	genCmd := []string{"ssh-keygen", "-t", "rsa", "-b", "4096",
		"-f", config.ClusterKeyPath, "-N", "", "-C", "cluster-key"}
	if err := n.podman.ContainerExecQuiet(ctx, n.ContainerName, genCmd); err != nil {
		return fmt.Errorf("generating SSH key: %w", err)
	}

	// Set correct permissions on private key (SSH requires 600)
	chmodCmd := []string{"chmod", "600", config.ClusterKeyPath}
	if err := n.podman.ContainerExecQuiet(ctx, n.ContainerName, chmodCmd); err != nil {
		return fmt.Errorf("setting key permissions: %w", err)
	}

	logrus.Infof("Cluster SSH key created at %s", config.ClusterKeyPath)
	return nil
}

func (n *Node) createOverlayDisk(ctx context.Context) error {
	overlayPath := fmt.Sprintf("/workspace/%s.qcow2", n.Name)

	logrus.Infof("Creating overlay disk for %s", n.Name)

	opts := &virsh.QemuImgCreateOptions{
		Path:          overlayPath,
		Format:        "qcow2",
		BackingFile:   n.BaseDisk,
		BackingFormat: "qcow2",
	}

	if err := n.virsh.QemuImgCreate(ctx, opts); err != nil {
		return fmt.Errorf("creating overlay disk: %w", err)
	}

	logrus.Infof("Overlay disk created at %s", overlayPath)
	return nil
}

func (n *Node) createVM(ctx context.Context) error {
	logrus.Infof("Creating VM %s", n.Name)

	overlayDisk := fmt.Sprintf("path=/workspace/%s.qcow2,format=qcow2,bus=virtio", n.Name)
	isoPath := fmt.Sprintf("path=/workspace/%s-cloud-init.iso,device=cdrom", n.Name)

	opts := &virsh.VirtInstallOptions{
		Name:   n.Name,
		Memory: n.Memory,
		VCPUs:  n.VCPUs,
		Disks:  []string{overlayDisk, isoPath},
		Networks: []virsh.NetworkConfig{
			{
				Type:        "passt",
				Model:       "virtio",
				PortForward: "2222:22",
			},
			{
				Type:  "mcast",
				Model: "virtio",
				MAC:   n.ClusterMAC,
			},
		},
		XMLModifications: []string{
			"xpath.set=./devices/interface[2]/source/@address=" + config.MulticastAddr,
			fmt.Sprintf("xpath.set=./devices/interface[2]/source/@port=%d", config.MulticastPort),
		},
	}

	if err := n.virsh.VirtInstall(ctx, opts); err != nil {
		return fmt.Errorf("creating VM with virt-install: %w", err)
	}

	logrus.Infof("VM %s created with dual-NIC networking", n.Name)
	logrus.Infof("  NIC 1 (enp1s0): passt - internet access")
	logrus.Infof("  NIC 2 (enp2s0): %s - cluster communication", n.ClusterIP)

	return nil
}
