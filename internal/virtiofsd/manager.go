package virtiofsd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bootc-dev/bink/internal/podman"
	"github.com/sirupsen/logrus"
)

// Manager handles the lifecycle of virtiofsd process
type Manager struct {
	containerName string
	nodeName      string
	podman        *podman.Client
	socketPath    string
	sharedDir     string
	pidFile       string
	isRunning     bool
}

// Options configures the virtiofsd manager
type Options struct {
	ContainerName string
	NodeName      string
	SharedDir     string
	SocketPath    string
	Cache         string   // auto, always, never
	Sandbox       string   // none, namespace, chroot
	ModCaps       string   // Capability modifications (e.g., "-mknod")
}

// NewManager creates a new virtiofsd manager
func NewManager(podman *podman.Client, opts *Options) *Manager {
	// Set defaults
	if opts.Cache == "" {
		opts.Cache = "auto"
	}
	if opts.Sandbox == "" {
		opts.Sandbox = "none"
	}

	pidFile := fmt.Sprintf("/tmp/virtiofsd-%s.pid", opts.NodeName)

	return &Manager{
		containerName: opts.ContainerName,
		nodeName:      opts.NodeName,
		podman:        podman,
		socketPath:    opts.SocketPath,
		sharedDir:     opts.SharedDir,
		pidFile:       pidFile,
		isRunning:     false,
	}
}

// Start begins the virtiofsd process in the background
func (m *Manager) Start(ctx context.Context, opts *Options) error {
	logrus.Infof("Starting virtiofsd for node %s", m.nodeName)

	// Create socket directory with qemu ownership
	socketDir := fmt.Sprintf("/var/lib/libvirt/qemu/domain-1-%s", m.nodeName)
	mkdirCmd := []string{"mkdir", "-p", socketDir}
	if err := m.podman.ContainerExecQuiet(ctx, m.containerName, mkdirCmd); err != nil {
		return fmt.Errorf("creating virtiofsd socket directory: %w", err)
	}

	// Change ownership to qemu user so virtiofsd can write to it
	chownCmd := []string{"chown", "qemu:qemu", socketDir}
	if err := m.podman.ContainerExecQuiet(ctx, m.containerName, chownCmd); err != nil {
		return fmt.Errorf("changing socket directory ownership: %w", err)
	}

	// Build virtiofsd command with proper backgrounding and PID tracking
	// Run as qemu user to avoid capability manipulation issues in containers
	virtiofsdCmd := fmt.Sprintf(
		"runuser -u qemu -- /usr/libexec/virtiofsd "+
			"--socket-path=%s "+
			"--shared-dir=%s "+
			"--cache=%s "+
			"--sandbox=%s",
		m.socketPath,
		m.sharedDir,
		opts.Cache,
		opts.Sandbox,
	)

	// Add modcaps if specified
	if opts.ModCaps != "" {
		virtiofsdCmd += fmt.Sprintf(" --modcaps=%s", opts.ModCaps)
	}

	// Add backgrounding and PID tracking
	// Use nohup to detach from terminal
	virtiofsdCmd = fmt.Sprintf(
		"nohup sh -c '%s' > /tmp/virtiofsd-%s.log 2>&1 & echo $! > %s",
		virtiofsdCmd,
		m.nodeName,
		m.pidFile,
	)

	// Execute the command to start virtiofsd in background
	shellCmd := []string{"sh", "-c", virtiofsdCmd}
	if err := m.podman.ContainerExecQuiet(ctx, m.containerName, shellCmd); err != nil {
		return fmt.Errorf("starting virtiofsd process: %w", err)
	}

	// Wait for socket to be created
	if err := m.waitForSocket(ctx); err != nil {
		return err
	}

	m.isRunning = true
	logrus.Infof("✓ virtiofsd ready at %s", m.socketPath)
	return nil
}

// waitForSocket polls for the socket file to be created
func (m *Manager) waitForSocket(ctx context.Context) error {
	logrus.Debug("Waiting for virtiofsd socket...")

	timeout := 10 * time.Second
	interval := 500 * time.Millisecond
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		checkCmd := []string{"test", "-S", m.socketPath}
		if err := m.podman.ContainerExecQuiet(ctx, m.containerName, checkCmd); err == nil {
			return nil
		}

		time.Sleep(interval)
	}

	// Socket wasn't created - get logs for debugging
	logsCmd := []string{"cat", fmt.Sprintf("/tmp/virtiofsd-%s.log", m.nodeName)}
	logs, _ := m.podman.ContainerExec(ctx, m.containerName, logsCmd)
	return fmt.Errorf("virtiofsd socket not created after %v. Logs:\n%s", timeout, logs)
}

// Stop terminates the virtiofsd process gracefully
func (m *Manager) Stop() error {
	if !m.isRunning {
		return nil
	}

	logrus.Debugf("Stopping virtiofsd for node %s", m.nodeName)

	ctx := context.Background()

	// Read PID from file
	pidCmd := []string{"cat", m.pidFile}
	pid, err := m.podman.ContainerExec(ctx, m.containerName, pidCmd)
	if err != nil {
		logrus.Warnf("Failed to read virtiofsd PID file: %v", err)
	} else {
		pid = strings.TrimSpace(pid)
		if pid != "" {
			// Kill the process gracefully (SIGTERM)
			killCmd := []string{"kill", pid}
			if err := m.podman.ContainerExecQuiet(ctx, m.containerName, killCmd); err != nil {
				logrus.Warnf("Failed to kill virtiofsd process %s: %v", pid, err)

				// Try force kill (SIGKILL)
				forceKillCmd := []string{"kill", "-9", pid}
				_ = m.podman.ContainerExecQuiet(ctx, m.containerName, forceKillCmd)
			}
		}
	}

	// Cleanup: remove socket file, PID file, and directory
	// This is best-effort cleanup
	if m.socketPath != "" {
		rmCmd := []string{"rm", "-f", m.socketPath}
		_ = m.podman.ContainerExecQuiet(ctx, m.containerName, rmCmd)
	}

	if m.pidFile != "" {
		rmPidCmd := []string{"rm", "-f", m.pidFile}
		_ = m.podman.ContainerExecQuiet(ctx, m.containerName, rmPidCmd)
	}

	m.isRunning = false
	return nil
}
