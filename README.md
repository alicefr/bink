# bink

A CLI tool for managing containerized Kubernetes clusters where each node is a Podman container running a VM inside.

## Compile

```bash
make build-bink
```

## Build Container Images

```bash
# Build all images (VM image, disk, and container images)
make all
```

## Create a Cluster

```bash
# Create cluster with control plane
./bink cluster start

# Add worker nodes (optional)
./bink node add node2
./bink node add node3

# Access the cluster
export KUBECONFIG=./kubeconfig
kubectl get nodes
```

## List Clusters

```bash
# List all running clusters
./bink cluster list
```

## Delete a Cluster

```bash
# Stop and remove all nodes
./bink cluster stop

# Stop and also remove persistent data (SSH keys, kubeconfig)
./bink cluster stop --remove-data
```
