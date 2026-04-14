package podman

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bootc-dev/bink/internal/util"
	"github.com/sirupsen/logrus"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	_, err := util.RunCommand(ctx, "podman", "network", "inspect", name)
	if err != nil {
		if execErr, ok := err.(*util.ExecError); ok && execErr.ExitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) NetworkCreate(ctx context.Context, name, subnet string) error {
	logrus.Infof("Creating podman network '%s' with subnet %s", name, subnet)

	_, err := util.RunCommand(ctx, "podman", "network", "create", name, "--subnet", subnet)
	if err != nil {
		if execErr, ok := err.(*util.ExecError); ok {
			if strings.Contains(execErr.Stderr, "already exists") {
				logrus.Infof("Network '%s' already exists", name)
				return nil
			}
			if strings.Contains(execErr.Stderr, "subnet") && strings.Contains(execErr.Stderr, "use") {
				logrus.Warnf("Subnet %s already in use, creating network with auto-assigned subnet", subnet)
				_, err = util.RunCommand(ctx, "podman", "network", "create", name)
				if err != nil {
					return fmt.Errorf("creating network with auto-assigned subnet: %w", err)
				}
			} else {
				return fmt.Errorf("creating network: %w", err)
			}
		} else {
			return fmt.Errorf("creating network: %w", err)
		}
	}

	logrus.Infof("Network '%s' created successfully", name)
	return nil
}

func (c *Client) NetworkInspect(ctx context.Context, name, format string) (string, error) {
	return util.RunCommandOutput(ctx, "podman", "network", "inspect", name, "--format", format)
}

func (c *Client) ContainerExists(ctx context.Context, name string) (bool, error) {
	_, err := util.RunCommand(ctx, "podman", "container", "exists", name)
	if err != nil {
		if execErr, ok := err.(*util.ExecError); ok && execErr.ExitCode != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) ContainerCreate(ctx context.Context, opts *ContainerCreateOptions) (string, error) {
	args := []string{"run", "-d", "--name", opts.Name}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	for _, device := range opts.Devices {
		args = append(args, "--device", device)
	}

	for _, volume := range opts.Volumes {
		args = append(args, "-v", volume)
	}

	for _, mount := range opts.Mounts {
		args = append(args, "--mount", mount)
	}

	for _, port := range opts.Ports {
		args = append(args, "-p", port)
	}

	for key, value := range opts.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	args = append(args, opts.Image)

	logrus.Debugf("Creating container with: podman %s", strings.Join(args, " "))
	return util.RunCommandOutput(ctx, "podman", args...)
}

func (c *Client) ContainerExec(ctx context.Context, name string, cmd []string) (string, error) {
	args := append([]string{"exec", name}, cmd...)
	return util.RunCommandOutput(ctx, "podman", args...)
}

func (c *Client) ContainerExecQuiet(ctx context.Context, name string, cmd []string) error {
	args := append([]string{"exec", name}, cmd...)
	return util.RunCommandQuiet(ctx, "podman", args...)
}

func (c *Client) ContainerExecInteractive(ctx context.Context, name string, cmd []string) error {
	args := append([]string{"exec", "-ti", name}, cmd...)

	logrus.Debugf("Executing interactively: podman %s", strings.Join(args, " "))

	result, err := util.RunCommand(ctx, "podman", args...)
	if err != nil {
		return err
	}

	if result.Stdout != "" {
		fmt.Println(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprintln(os.Stderr, result.Stderr)
	}

	return nil
}

func (c *Client) ContainerStop(ctx context.Context, name string) error {
	logrus.Debugf("Stopping container %s", name)
	return util.RunCommandQuiet(ctx, "podman", "stop", name)
}

func (c *Client) ContainerRemove(ctx context.Context, name string, force bool) error {
	logrus.Debugf("Removing container %s", name)
	args := []string{"rm", name}
	if force {
		args = append([]string{"rm", "-f"}, name)
	}
	return util.RunCommandQuiet(ctx, "podman", args...)
}

func (c *Client) ContainerList(ctx context.Context, filter string) ([]string, error) {
	args := []string{"ps", "-a", "--format", "{{.Names}}"}
	if filter != "" {
		args = append(args, "--filter", filter)
	}

	output, err := util.RunCommandOutput(ctx, "podman", args...)
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	return strings.Split(output, "\n"), nil
}

func (c *Client) ContainerInspect(ctx context.Context, name, format string) (string, error) {
	return util.RunCommandOutput(ctx, "podman", "inspect", "-f", format, name)
}

func (c *Client) ContainerCopy(ctx context.Context, srcPath, containerName, destPath string) error {
	dest := fmt.Sprintf("%s:%s", containerName, destPath)
	return util.RunCommandQuiet(ctx, "podman", "cp", srcPath, dest)
}

func (c *Client) VolumeRemove(ctx context.Context, name string) error {
	logrus.Debugf("Removing volume %s", name)
	return util.RunCommandQuiet(ctx, "podman", "volume", "rm", name)
}
