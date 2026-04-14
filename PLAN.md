# Implementation Plan: bink - Containerized Kubernetes Clusters

## Infrastructure Overview

**bink** manages multi-node Kubernetes clusters where each node runs as a rootless Podman container hosting a bootc-based VM. This architecture delivers VM isolation with container convenience.

### Networking
- **passt**: Each VM's first NIC (enp1s0) provides internet access via userspace networking
- **multicast**: Second NIC (enp2s0) connects to cluster network (10.0.0.0/24) for Kubernetes communication
- IP allocation: MD5 hash of node name → `10.0.0.{hash % 240 + 10}`

### Storage
- **Base VM images**: QCOW2 disks from bootc images, mounted read-only via container image volumes at `/images`
- **Overlay disks**: Per-node QCOW2 overlays in ephemeral container storage (`/workspace/{node}.qcow2`)
- **Container images**: Shared `cluster-images` volume pre-populated with Kubernetes images

### Container Image Caching
- **virtiofs**: Mounts `cluster-images` volume into each VM at `/var/lib/containers/storage` 
- Pre-pulled K8s images (apiserver, scheduler, controller-manager, etcd, coredns) available immediately
- Eliminates redundant image pulls across cluster nodes

### Architecture Diagram

**Multi-Cluster Setup**
```
┌───────────────────────────────────────────────────────────────┐
│ Host System                                                   │
│                                                               │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │ Cluster "cluster1"                                       │ │
│  │                                                          │ │
│  │  ┌──────────────────┐      ┌──────────────────┐          │ │
│  │  │ Container        │      │ Container        │          │ │
│  │  │ k8s-node1        │══════│ k8s-node2        │          │ │
│  │  │                  │      │                  │          │ │
│  │  │  ┌────────────┐  │      │  ┌────────────┐  │          │ │
│  │  │  │ VM: node1  │  │      │  │ VM: node2  │  │          │ │
│  │  │  │ 10.0.0.32  │  │      │  │ 10.0.0.130 │  │          │ │
│  │  │  └────────────┘  │      │  └────────────┘  │          │ │
│  │  └──────────────────┘      └──────────────────┘          │ │
│  └──────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌────────────────────────┐                                   │
│  │ Cluster "cluster2"     │                                   │
│  │                        │                                   │
│  │  ┌──────────────────┐  │                                   │
│  │  │ Container        │  │                                   │
│  │  │ k8s-test-node1   │  │                                   │
│  │  │                  │  │                                   │
│  │  │  ┌────────────┐  │  │                                   │
│  │  │  │ VM: node1  │  │  │                                   │
│  │  │  │ 10.0.0.32  │  │  │                                   │
│  │  │  └────────────┘  │  │                                   │
│  │  └──────────────────┘  │                                   │
│  └────────────────────────┘                                   │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

## Podman Integration

bink uses Podman Go bindings (`github.com/containers/podman/v6/pkg/bindings`) for all container operations.

