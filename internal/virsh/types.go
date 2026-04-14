package virsh

type VirtInstallOptions struct {
	Name             string
	Memory           int
	VCPUs            int
	Disks            []string
	Networks         []NetworkConfig
	XMLModifications []string
}

type NetworkConfig struct {
	Type        string
	Model       string
	MAC         string
	PortForward string
}

type QemuImgCreateOptions struct {
	Path          string
	Format        string
	BackingFile   string
	BackingFormat string
	Size          string
}
