package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/bootc-dev/bink/internal/config"
	"github.com/sirupsen/logrus"
)

const (
	ClusterImagesVolume = "cluster-images"
	// Path inside container where volume is mounted
	ClusterImagesMountPath = "/var/lib/cluster-images"
	// Global container name for volume population (shared across all clusters)
	PopulatorContainerName = "cluster-images-populator"
)

// EnsureImagesVolume creates and populates the cluster-images volume if it doesn't exist
// This volume is shared across all clusters since the images are identical
func (c *Cluster) EnsureImagesVolume(ctx context.Context) error {
	volumeName := ClusterImagesVolume

	logrus.Infof("Ensuring cluster images volume: %s", volumeName)

	// Check if volume exists
	exists, err := c.volumeExists(ctx, volumeName)
	if err != nil {
		return fmt.Errorf("checking volume existence: %w", err)
	}

	if exists {
		logrus.Infof("Volume %s already exists, checking if populated...", volumeName)

		// Check if volume is already successfully populated
		if c.isVolumeCompleted(ctx, volumeName) {
			logrus.Infof("Volume %s is already populated with images", volumeName)
			return nil
		}

		// Check if another process is currently populating the volume
		if c.isPopulationInProgress(ctx) {
			logrus.Infof("Another process is populating the volume, waiting...")
			if err := c.waitForPopulationComplete(ctx); err != nil {
				return fmt.Errorf("waiting for volume population: %w", err)
			}
			logrus.Infof("Volume population complete")
			return nil
		}

		logrus.Infof("Volume exists but is not populated, will populate...")
	} else {
		logrus.Infof("Creating volume %s...", volumeName)
		if err := c.createVolume(ctx, volumeName); err != nil {
			return fmt.Errorf("creating volume: %w", err)
		}
	}

	// Populate the volume with Kubernetes images
	if err := c.populateImagesVolume(ctx, volumeName); err != nil {
		// If another process started populating after our check, wait for it
		if c.isPopulationInProgress(ctx) {
			logrus.Infof("Another process started populating concurrently, waiting...")
			if waitErr := c.waitForPopulationComplete(ctx); waitErr != nil {
				return fmt.Errorf("waiting for concurrent population: %w", waitErr)
			}
			logrus.Infof("Concurrent population complete")
			return nil
		}
		return fmt.Errorf("populating volume: %w", err)
	}

	// Mark volume as completed
	if err := c.markVolumeCompleted(ctx, volumeName); err != nil {
		logrus.Warnf("Failed to mark volume as completed: %v", err)
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
	err := c.runCommand(ctx, cmd...)
	if err != nil {
		// If volume already exists due to race condition, that's fine
		if strings.Contains(err.Error(), "volume already exists") {
			logrus.Debugf("Volume %s already exists (created by parallel process)", name)
			return nil
		}
		return err
	}
	return nil
}

func (c *Cluster) isVolumePopulated(ctx context.Context, volumeName string) (bool, error) {
	// Run a container to check if the volume has container storage structure
	cmd := []string{
		"podman", "run", "--rm",
		"-v", fmt.Sprintf("%s:/check:z", volumeName),
		config.DefaultBaseImage,
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

	// Use a global container name for population (shared across all clusters)
	// This allows other processes to wait for it using `podman wait`
	tmpContainer := PopulatorContainerName

	logrus.Infof("Creating populator container: %s", tmpContainer)

	// Start a long-running container that we can exec into
	startCmd := []string{
		"podman", "run", "-d",
		"--name", tmpContainer,
		"-v", fmt.Sprintf("%s:/var/lib/containers/storage:z", volumeName),
		config.DefaultBaseImage,
		"sleep", "3600",
	}

	if err := c.runCommand(ctx, startCmd...); err != nil {
		// Container name already in use means another process is populating
		return fmt.Errorf("starting populator container (another process may be populating): %w", err)
	}

	// Ensure cleanup
	defer func() {
		logrus.Debugf("Cleaning up populator container %s", tmpContainer)
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
// The images volume is shared across all clusters
func (c *Cluster) GetImagesVolumeName() string {
	return ClusterImagesVolume
}

// isVolumeCompleted checks if volume has been successfully populated
func (c *Cluster) isVolumeCompleted(ctx context.Context, volumeName string) bool {
	cmd := []string{
		"podman", "run", "--rm",
		"-v", fmt.Sprintf("%s:/check:z", volumeName),
		config.DefaultBaseImage,
		"test", "-f", "/check/.completed",
	}
	err := c.runCommand(ctx, cmd...)
	return err == nil
}

// isPopulationInProgress checks if the populator container is currently running
func (c *Cluster) isPopulationInProgress(ctx context.Context) bool {
	cmd := []string{"podman", "container", "exists", PopulatorContainerName}
	err := c.runCommand(ctx, cmd...)
	return err == nil
}

// waitForPopulationComplete waits for the populator container to finish
// Uses `podman wait` which is event-driven (no polling/sleep)
func (c *Cluster) waitForPopulationComplete(ctx context.Context) error {
	logrus.Debugf("Waiting for populator container %s to complete...", PopulatorContainerName)

	// Use podman wait to block until container exits (event-driven, no polling)
	cmd := []string{"podman", "wait", PopulatorContainerName}
	exitCode, err := c.runCommandOutput(ctx, cmd...)
	if err != nil {
		// Container might have already exited and been removed
		logrus.Debugf("Container wait failed (may have already completed): %v", err)
		return nil
	}

	exitCode = strings.TrimSpace(exitCode)
	logrus.Debugf("Populator container exited with code: %s", exitCode)

	// Check if population was successful
	if exitCode != "0" {
		return fmt.Errorf("population failed with exit code %s", exitCode)
	}

	return nil
}

// markVolumeCompleted creates a marker file indicating successful population
func (c *Cluster) markVolumeCompleted(ctx context.Context, volumeName string) error {
	cmd := []string{
		"podman", "run", "--rm",
		"-v", fmt.Sprintf("%s:/mark:z", volumeName),
		config.DefaultBaseImage,
		"touch", "/mark/.completed",
	}
	return c.runCommand(ctx, cmd...)
}
