package virsh

import (
	"context"
	"fmt"
	"strings"

	"github.com/bootc-dev/bink/internal/util"
	"github.com/sirupsen/logrus"
)

type Client struct {
	containerName string
}

func NewClient(containerName string) *Client {
	return &Client{
		containerName: containerName,
	}
}

func (c *Client) ExecInContainer(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"exec", c.containerName}, args...)
	return util.RunCommandOutput(ctx, "podman", fullArgs...)
}

func (c *Client) VirtInstall(ctx context.Context, opts *VirtInstallOptions) error {
	args := []string{
		"virt-install",
		"--connect", "qemu:///session",
		"--name", opts.Name,
		"--memory", fmt.Sprintf("%d", opts.Memory),
		"--vcpus", fmt.Sprintf("%d", opts.VCPUs),
		"--import",
		"--os-variant", "fedora-unknown",
		"--graphics", "none",
		"--console", "pty,target_type=serial",
		"--noautoconsole",
	}

	for _, disk := range opts.Disks {
		args = append(args, "--disk", disk)
	}

	for _, network := range opts.Networks {
		netArg := network.Type
		if network.Model != "" {
			netArg += fmt.Sprintf(",model=%s", network.Model)
		}
		if network.MAC != "" {
			netArg += fmt.Sprintf(",mac=%s", network.MAC)
		}
		if network.PortForward != "" {
			netArg += fmt.Sprintf(",portForward=%s", network.PortForward)
		}
		args = append(args, "--network", netArg)
	}

	for _, fs := range opts.Filesystems {
		// Build filesystem argument for virt-install
		// Format: type=mount,source=/path,target=tag,mode=mapped,readonly=on
		fsArg := "type=mount"
		fsArg += fmt.Sprintf(",source=%s", fs.Source)
		fsArg += fmt.Sprintf(",target=%s", fs.Target)

		if fs.AccessMode != "" {
			fsArg += fmt.Sprintf(",accessmode=%s", fs.AccessMode)
		} else {
			fsArg += ",accessmode=passthrough"
		}

		if fs.ReadOnly {
			fsArg += ",readonly=on"
		}

		args = append(args, "--filesystem", fsArg)
	}

	for _, xml := range opts.XMLModifications {
		args = append(args, "--xml", xml)
	}

	args = append(args, "--channel", "unix,target.type=virtio,target.name=org.qemu.guest_agent.0")

	logrus.Debugf("Creating VM with virt-install: %s", strings.Join(args, " "))

	fullArgs := append([]string{"exec", c.containerName}, args...)
	return util.RunCommandQuiet(ctx, "podman", fullArgs...)
}

func (c *Client) QemuImgCreate(ctx context.Context, opts *QemuImgCreateOptions) error {
	args := []string{
		"qemu-img", "create",
		"-f", opts.Format,
	}

	if opts.BackingFile != "" {
		args = append(args, "-F", opts.BackingFormat, "-b", opts.BackingFile)
	}

	args = append(args, opts.Path)

	if opts.Size != "" {
		args = append(args, opts.Size)
	}

	logrus.Debugf("Creating disk image: qemu-img %s", strings.Join(args, " "))

	fullArgs := append([]string{"exec", c.containerName}, args...)
	return util.RunCommandQuiet(ctx, "podman", fullArgs...)
}

func (c *Client) Genisoimage(ctx context.Context, outputPath, volumeID string, files []string) error {
	args := []string{
		"genisoimage",
		"-output", outputPath,
		"-volid", volumeID,
		"-joliet",
		"-rock",
	}

	args = append(args, files...)

	logrus.Debugf("Creating ISO: genisoimage %s", strings.Join(args, " "))

	fullArgs := append([]string{"exec", c.containerName}, args...)
	return util.RunCommandQuiet(ctx, "podman", fullArgs...)
}

func (c *Client) DomainList(ctx context.Context) ([]string, error) {
	output, err := c.ExecInContainer(ctx, "virsh", "-c", "qemu:///session", "list", "--name", "--all")
	if err != nil {
		return nil, err
	}

	if output == "" {
		return []string{}, nil
	}

	domains := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			domains = append(domains, line)
		}
	}

	return domains, nil
}

func (c *Client) DomainDestroy(ctx context.Context, name string) error {
	logrus.Debugf("Destroying domain %s", name)
	return util.RunCommandQuiet(ctx, "podman", "exec", c.containerName,
		"virsh", "-c", "qemu:///session", "destroy", name)
}

func (c *Client) DomainUndefine(ctx context.Context, name string) error {
	logrus.Debugf("Undefining domain %s", name)
	return util.RunCommandQuiet(ctx, "podman", "exec", c.containerName,
		"virsh", "-c", "qemu:///session", "undefine", name)
}
