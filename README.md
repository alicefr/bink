# bink

A CLI tool for managing containerized Kubernetes clusters where each node is a Podman container running a VM inside.

## Overview

bink combines the isolation benefits of VMs with the convenience of containers to create lightweight, ephemeral Kubernetes clusters for development and testing. Each cluster node runs as a Podman container that hosts a bootc-based VM internally.

## Quick Start

```bash
# Build the project
make build-bink

# Create a cluster (control plane)
./bink cluster start

# Add worker nodes
./bink node add node2
./bink node add node3

# Access a node
./bink node ssh node1

# List all nodes
./bink node list

# Expose the API server
./bink api expose

# Use kubectl
export KUBECONFIG=./kubeconfig
kubectl get nodes

# Clean up
./bink cluster stop
```

## Commands

- `bink cluster start` - Create cluster with control plane
- `bink cluster stop` - Stop and remove all nodes
- `bink cluster stop --remove-data` - Also remove persistent data (SSH keys, kubeconfig)
- `bink node add <name>` - Create and join worker node
- `bink node ssh <name>` - SSH into a node's VM
- `bink node list` - List all cluster nodes
- `bink api expose` - Expose API server via SSH tunnel

## Architecture

**Storage:**
- Base VM images: Packaged in container images, mounted as read-only volumes
- Node disks: Ephemeral overlay storage (auto-cleaned on container removal)
- SSH keys: Named Podman volume `cluster-keys` (persists across restarts)

**Networking:**
- Libvirt network (10.88.0.0/16): VM-to-container communication
- Cluster network (10.0.0.0/24): Inter-node Kubernetes traffic via multicast
- Port forwarding: Host → container:6443 → VM:6443 for API access

## Building

```bash
# Build VM image from bootc
make build-vm-image

# Convert to qcow2 disk
make build-disk

# Package as container image
make build-images-container

# Build cluster container image
make build-cluster-image

# Build bink binary
make build-bink

# All-in-one
make all
```

## Requirements

- Podman
- Go 1.25+
- bootc-image-builder (bcvk) - for building VM images

libvirt/qemu and other VM dependencies are bundled in the container images.

## Development

```bash
cd cmd/bink
go build -o ../../bink

# Run tests
go test ./...
```

## License

See LICENSE file for details.
