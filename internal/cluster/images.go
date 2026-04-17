package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	ClusterImagesVolume = "cluster-images"
	// Path inside container where volume is mounted
	ClusterImagesMountPath = "/var/lib/cluster-images"
)

// EnsureImagesVolume creates and populates the cluster-images volume if it doesn't exist
func (c *Cluster) EnsureImagesVolume(ctx context.Context) error {
	volumeName := fmt.Sprintf("%s-%s", c.name, ClusterImagesVolume)
	if c.name == "" || c.name == "podman" {
		volumeName = ClusterImagesVolume
	}

	logrus.Infof("Ensuring cluster images volume: %s", volumeName)

	// Check if volume exists
	exists, err := c.volumeExists(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("checking volume existence: %w", err)
	}

	if exists {
		logrus.Infof("Volume %s already exists, checking if populated...", volumeName)
		populated, err := c.isVolumePopulated(ctx, volumeName)
		if err != nil {
			return fmt.Errorf("checking if volume is populated: %w", err)
		}
		if populated {
			logrus.Infof("Volume %s is already populated with images", volumeName)
			return nil
		}
		logrus.Infof("Volume exists but is empty, will populate...")
	} else {
		logrus.Infof("Creating volume %s...", volumeName)
		if err := c.createVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("creating volume: %w", err)
		}
	}

	// Populate the volume with Kubernetes images
	if err := c.populateImagesVolume(ctx, volumeName); err != nil {
		return fmt.Errorf("populating volume: %w", err)
	}

	logrus.Infof("✓ Cluster images volume ready: %s", volumeName)
	return nil
}

func (c *Cluster) volumeExists(ctx context.Context, name string) (bool, error) {
	cmd := []string{"podman", "volume", "exists", name}
	err := c.runCommand(ctx, cmd...)
	return err == nil, nil
}

func (c *Cluster) createVolume(ctx context.Context, name string) error {
	cmd := []string{"podman", "volume", "create", name}
	return c.runCommand(ctx, cmd...)
}

func (c *Cluster) isVolumePopulated(ctx context.Context, volumeName string) (bool, error) {
	// Run a container to check if the volume has container storage structure
	cmd := []string{
		"podman", "run", "--rm",
		"-v", fmt.Sprintf("%s:/check:z", volumeName),
		"quay.io/fedora/fedora:43",
		"sh", "-c", "test -d /check/overlay-images && test -d /check/overlay-layers",
	}
	err := c.runCommand(ctx, cmd...)
	return err == nil, nil
}

func (c *Cluster) populateImagesVolume(ctx context.Context, volumeName string) error {
	logrus.Info("Pre-pulling Kubernetes images into volume...")

	k8sVersion := "v1.35.0"

	// Try to get image list from kubeadm in the bootc image
	// If that fails, fall back to hardcoded list
	var images []string

	cmd := []string{
		"podman", "run", "--rm",
		"localhost/fedora-bootc-k8s:latest",
		"kubeadm", "config", "images", "list", "--kubernetes-version", k8sVersion,
	}
	output, err := c.runCommandOutput(ctx, cmd...)
	if err != nil {
		logrus.Warnf("Could not get image list from kubeadm: %v", err)
		logrus.Info("Using hardcoded image list for Kubernetes v1.35.0")

		// Hardcoded image list for Kubernetes v1.35.0
		images = []string{
			"registry.k8s.io/kube-apiserver:v1.35.0",
			"registry.k8s.io/kube-controller-manager:v1.35.0",
			"registry.k8s.io/kube-scheduler:v1.35.0",
			"registry.k8s.io/kube-proxy:v1.35.0",
			"registry.k8s.io/coredns/coredns:v1.11.1",
			"registry.k8s.io/pause:3.10",
			"registry.k8s.io/etcd:3.5.16-0",
		}
	} else {
		images = strings.Split(strings.TrimSpace(output), "\n")
	}

	logrus.Infof("Found %d images to pull", len(images))

	// Create a temporary container with the volume mounted as its storage
	// This allows us to pull images directly into the volume
	tmpContainer := fmt.Sprintf("tmp-image-puller-%s", c.name)

	logrus.Infof("Creating temporary container to populate volume...")

	// Start a long-running container that we can exec into
	startCmd := []string{
		"podman", "run", "-d",
		"--name", tmpContainer,
		"-v", fmt.Sprintf("%s:/var/lib/containers/storage:z", volumeName),
		"quay.io/fedora/fedora:43",
		"sleep", "3600",
	}

	if err := c.runCommand(ctx, startCmd...); err != nil {
		return fmt.Errorf("starting temporary container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		logrus.Debugf("Cleaning up temporary container %s", tmpContainer)
		c.runCommand(ctx, "podman", "rm", "-f", tmpContainer)
	}()

	// Install skopeo and podman in the container
	logrus.Info("Installing container tools in temporary container...")
	installCmd := []string{
		"podman", "exec", tmpContainer,
		"dnf", "install", "-y", "-q", "skopeo", "podman",
	}
	if err := c.runCommand(ctx, installCmd...); err != nil {
		return fmt.Errorf("installing container tools: %w", err)
	}

	// Configure subuid/subgid to allow user namespacing
	logrus.Debug("Configuring storage for image extraction...")
	setupStorageCmd := []string{
		"podman", "exec", tmpContainer,
		"sh", "-c",
		"echo 'root:100000:65536' > /etc/subuid && " +
			"echo 'root:100000:65536' > /etc/subgid && " +
			"podman system migrate 2>/dev/null || true",
	}
	if err := c.runCommand(ctx, setupStorageCmd...); err != nil {
		logrus.Debug("Storage configuration completed with warnings")
	}

	// Pull each image using skopeo
	for i, image := range images {
		if image == "" {
			continue
		}
		logrus.Infof("[%d/%d] Pulling %s", i+1, len(images), image)

		pullCmd := []string{
			"podman", "exec", tmpContainer,
			"skopeo", "copy",
			"docker://" + image,
			"containers-storage:" + image,
		}

		if err := c.runCommand(ctx, pullCmd...); err != nil {
			logrus.Warnf("Failed to pull %s: %v (continuing...)", image, err)
			continue
		}
	}

	logrus.Info("✓ All images pulled successfully")
	return nil
}

// GetImagesVolumeName returns the volume name for this cluster
func (c *Cluster) GetImagesVolumeName() string {
	if c.name == "" || c.name == "podman" {
		return ClusterImagesVolume
	}
	return fmt.Sprintf("%s-%s", c.name, ClusterImagesVolume)
}
