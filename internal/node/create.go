package node

import (
	"context"
	"fmt"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/bootc-dev/bink/internal/podman"
	"github.com/bootc-dev/bink/internal/virsh"
	"github.com/bootc-dev/bink/internal/virtiofsd"
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

	// Determine cluster images volume name
	clusterImagesVolume := "cluster-images"
	if n.ClusterName != "" && n.ClusterName != "podman" {
		clusterImagesVolume = fmt.Sprintf("%s-cluster-images", n.ClusterName)
	}

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
			fmt.Sprintf("%s:/var/lib/cluster-images:z,ro", clusterImagesVolume),
		},
		// Enable privileges needed for virtiofsd
		CapAdd:      []string{"SYS_ADMIN"},
		SecurityOpt: []string{"label=disable"},
	}

	if n.IsControlPlane {
		// Use configured API port (0 = auto-assign random port)
		var portMapping string
		if n.APIPort == 0 {
			// Empty host port means auto-assign
			portMapping = ":6443"
		} else {
			portMapping = fmt.Sprintf("%d:6443", n.APIPort)
		}
		opts.Ports = []string{portMapping}
	}

	containerID, err := n.podman.ContainerCreate(ctx, opts)
	if err != nil {
		return fmt.Errorf("creating container: %w", err)
	}

	logrus.Infof("Container %s created: %s", n.ContainerName, containerID)

	// If using auto-assigned port (APIPort=0), get the actual assigned port
	if n.IsControlPlane && n.APIPort == 0 {
		assignedPort, err := n.podman.GetPublishedPort(ctx, n.ContainerName, "6443/tcp")
		if err != nil {
			return fmt.Errorf("getting assigned API port: %w", err)
		}
		n.AssignedAPIPort = assignedPort
		logrus.Infof("API server port auto-assigned: %d", assignedPort)
	} else if n.IsControlPlane {
		n.AssignedAPIPort = n.APIPort
		logrus.Infof("API server port: %d", n.AssignedAPIPort)
	}

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

func (n *Node) setupVirtiofsd(ctx context.Context) error {
	logrus.Info("Setting up virtiofsd for cluster images")

	// Socket path matches what libvirt expects
	socketDir := fmt.Sprintf("/var/lib/libvirt/qemu/domain-1-%s", n.Name)
	socketPath := fmt.Sprintf("%s/fs0-fs.sock", socketDir)

	// Create virtiofsd manager with options based on podman-bootc approach
	opts := &virtiofsd.Options{
		ContainerName: n.ContainerName,
		NodeName:      n.Name,
		SharedDir:     "/var/lib/cluster-images",
		SocketPath:    socketPath,
		Cache:         "auto",
		Sandbox:       "none",
		ModCaps:       "-mknod", // Disable mknod capability to avoid kernel sync errors
	}

	n.virtiofsdMgr = virtiofsd.NewManager(n.podman, opts)

	// Start the virtiofsd process with proper lifecycle management
	if err := n.virtiofsdMgr.Start(ctx, opts); err != nil {
		return fmt.Errorf("starting virtiofsd: %w", err)
	}

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
		Filesystems: []virsh.FilesystemConfig{
			{
				Source:     "/var/lib/cluster-images",
				Target:     "cluster_images",
				AccessMode: "passthrough",
				ReadOnly:   true,
			},
		},
		XMLModifications: []string{
			"xpath.set=./devices/interface[2]/source/@address=" + config.MulticastAddr,
			fmt.Sprintf("xpath.set=./devices/interface[2]/source/@port=%d", config.MulticastPort),
			// Set socket type and path for externally managed virtiofsd
			fmt.Sprintf("xpath.set=./devices/filesystem/source/@socket=/var/lib/libvirt/qemu/domain-1-%s/fs0-fs.sock", n.Name),
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
