package podman

type ContainerCreateOptions struct {
	Name        string
	Image       string
	Network     string
	Devices     []string
	Volumes     []string
	Mounts      []string
	Ports       []string
	Environment map[string]string
}
