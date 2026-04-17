# Integration Test Plan: Bink Cluster Management

**Framework:** Ginkgo v2 + Gomega  
**Target:** End-to-end validation of cluster creation, management, and teardown  
**Created:** 2026-04-16  
**Last Updated:** 2026-04-17  
**Status:** Active - In Use

## Changelog

**2026-04-17:**
- ✅ **Implemented port randomization** - Tests now use `--api-port 0` for auto-assigned ports
- ✅ **Enabled parallel test execution** - Removed `Serial` markers, tests can run in parallel
- ✅ **Added test merging principles for efficiency**
- ✅ Merged single-node cluster tests into comprehensive test (reduces test time by ~6-8 min)
- ✅ Merged cluster stop isolation tests into single comprehensive test
- ✅ Added cluster stop isolation test to validate cluster name-based separation
- ✅ Updated test design principles with "When to Split vs Merge" guidance
- ✅ Documented time savings from test merging strategy

**2026-04-16:**
- Initial draft created

---

## Table of Contents

1. [Overview](#overview)
2. [Test Framework Setup](#test-framework-setup)
3. [Test Scenarios](#test-scenarios)
4. [Test Helpers & Utilities](#test-helpers--utilities)
5. [Execution Strategy](#execution-strategy)
6. [Open Questions](#open-questions)

---

## Overview

### Objectives

- **Validate cluster lifecycle**: Creation, node addition, API exposure, and teardown
- **Ensure reliability**: Test edge cases, error handling, and recovery scenarios
- **Prevent regressions**: Automated testing on every PR
- **Document behavior**: Tests serve as executable documentation

### Scope

**In Scope:**
- Cluster creation (single-node and multi-node)
- Node operations (add, list, SSH access)
- API server exposure
- DNS management
- SSH key management
- Error scenarios and cleanup

**Out of Scope:**
- Kubernetes workload testing (covered by k8s e2e tests)
- Performance/load testing (separate suite)
- UI/UX testing
- Network implementation details
- Storage layer details

### Test Design Principles

**Test Merging for Efficiency:**
- **Merge related tests** that validate different aspects of the same operation
- Tests that share the same cluster setup/teardown should be combined into a single comprehensive test
- Use multiple `By()` steps within a single test to document different validation stages
- **Example:** Instead of separate tests for "create cluster", "verify k8s", "verify CNI", "verify DNS" - merge into one test with multiple verification stages
- **Benefit:** Saves cluster creation/destruction time, making test suite run 3-4x faster

**Atomic Test Isolation:**
- Each test must be completely isolated and independent
- Use unique, randomized cluster names (e.g., `test-bink-abc123`) for each test
- Cleanup all resources in `AfterEach` to prevent test pollution
- Tests must be safe to run in parallel without interference
- No shared state between tests

**When to Split vs Merge:**
- **Merge:** Tests validating different outputs/states of the same command execution
- **Split:** Tests requiring different input parameters or testing different code paths
- **Split:** Tests that need to verify failure scenarios vs success scenarios

**Rationale:**
- Merging reduces redundant cluster creation (2min+ per cluster)
- Enables parallel execution for faster feedback
- Makes tests more reliable and reproducible
- Easier to debug failures when all related validations are in one place
- Prevents cascading failures from one test affecting others

### Test Environment Requirements

- **Host OS:** Linux (Fedora 42+ recommended)
- **Dependencies:** Podman only (libvirt, virsh, qemu are inside container images)
- **Permissions:** Ability to create containers, networks, and volumes
- **Resources:** 16GB RAM, 50GB disk (for multi-node tests)
- **Network:** No conflicting networks on 10.0.0.0/24 or 10.88.0.0/16

---

## Test Framework Setup

### Dependencies

- `github.com/onsi/ginkgo/v2` - Test framework
- `github.com/onsi/gomega` - Assertion library
- `github.com/google/uuid` - Unique test cluster names

### Suite Setup Requirements

**Before Suite:**
- Verify podman is available
- Verify bink binary exists in project root
- Verify test images exist (cluster image, VM images container)

**After Suite:**
- Global cleanup of any leftover test resources
- Remove test clusters with prefix `test-bink-*`

---

## Test Scenarios

### 1. Cluster Lifecycle Tests

**File:** `test/integration/cluster_test.go`

#### 1.1 Single-Node Cluster Creation

**Test: Create and initialize a complete Kubernetes cluster** _(Merged test - all stages of cluster start)_
- Execute: `bink cluster start --cluster-name <unique-name>`
- Verify: Cluster creation command succeeded
- Verify: Container exists and is running
- Verify: Port 6443 is published
- Verify: Kubernetes is initialized and node is Ready
- Verify: Node has `control-plane` role
- Verify: Calico CNI pods are running in kube-system namespace
- Verify: DNS (dnsmasq) is configured and running
- Verify: cluster-hosts file contains node1 entry with correct IP (10.0.0.32)
- Expected: Fully functional Kubernetes cluster with networking and DNS
- **Note:** This test merges container creation, k8s init, CNI, and DNS verification into a single test because they all validate different aspects of the same `cluster start` command. Saves ~6-8 minutes compared to running 4 separate tests.

**Test: Handle cluster already exists error**
- Given: Cluster already created
- Execute: `bink cluster start` again with same cluster name
- Verify: Command fails with appropriate error
- Verify: Error message mentions "already exists"
- Expected: Clear error message about existing cluster

#### 1.2 Multi-Node Cluster Creation

**Test: Add worker node successfully**
- Given: Cluster with control plane
- Execute: `bink node add node2 --role worker`
- Verify: Container created and running
- Verify: Node joins cluster and becomes Ready
- Verify: Node has `worker` role in kubectl output
- Expected: Worker node joins successfully

**Test: Label worker nodes with worker role**
- Given: Cluster with control plane
- Execute: Add worker node
- Verify: Worker role label applied automatically
- Verify: Label `node-role.kubernetes.io/worker=worker` exists
- Verify: Role appears in `kubectl get nodes` ROLES column
- Expected: Worker nodes automatically labeled

**Test: Add control-plane node successfully**
- Given: Cluster with control plane
- Execute: `bink node add node2 --role control-plane`
- Verify: Container created with port 6443 published
- Verify: Node joins cluster as control-plane
- Verify: Node has `control-plane` role in kubectl output
- Verify: Control-plane label exists
- Expected: Additional control-plane node joins successfully

**Test: Create HA cluster with multiple control-plane nodes**
- Given: Cluster with control plane
- Execute: Add node2 and node3 as control-plane
- Verify: All three nodes show as control-plane
- Verify: All three nodes are Ready
- Verify: etcd has 3 members
- Expected: HA cluster with 3 control-plane nodes

**Test: Update DNS entries when adding nodes**
- Given: Cluster with control plane
- Execute: Add worker node2
- Verify: DNS entry added to cluster-hosts file
- Verify: Entry maps node2 to 10.0.0.130
- Verify: dnsmasq reloaded
- Expected: DNS updated automatically

**Test: Allow pods to schedule on worker nodes**
- Given: Cluster with control plane and worker
- Execute: Deploy nginx pod
- Verify: Pod transitions to Running state
- Verify: Pod can be scheduled on any node
- Expected: Workloads run on worker nodes

#### 1.3 Cluster Teardown

**Test: Isolate cluster stop operations by cluster name** _(Merged test - cluster stop isolation)_
- Execute: Create a named cluster
- Verify: Cluster exists
- Execute: Attempt to stop a different (non-existent) cluster with `--cluster-name`
- Verify: Stop command succeeds (no containers to stop)
- Verify: Original cluster still exists (other cluster stop didn't affect it)
- Execute: Attempt to stop default cluster (without `--cluster-name`)
- Verify: Default stop command succeeds
- Verify: Named cluster still exists (default stop should only affect `k8s-node*` containers)
- Execute: Stop the correct cluster by name
- Verify: Cluster is now stopped and removed
- Expected: Cluster stop operations are properly isolated by cluster name
- **Note:** This test verifies that stopping one cluster doesn't affect others, and that default vs named cluster namespaces are properly separated.

**Test: Stop and remove all containers**
- Given: Cluster with multiple nodes
- Execute: `bink cluster stop --cluster-name <cluster>`
- Verify: All node containers for that cluster removed
- Verify: Containers no longer appear in `podman ps -a`
- Expected: Clean cluster shutdown

**Test: Preserve SSH keys volume by default**
- Given: Cluster created and stopped
- Execute: `bink cluster stop`
- Verify: cluster-keys volume still exists
- Expected: SSH keys preserved for cluster restart

**Test: Remove all data with --remove-data flag**
- Given: Cluster with kubeconfig exposed
- Execute: `bink cluster stop --remove-data --cluster-name <cluster>`
- Verify: cluster-keys volume removed
- Verify: kubeconfig file removed
- Expected: Complete cleanup including persistent data

---

### 2. Node Operations Tests

**File:** `test/integration/node_test.go`

#### 2.1 Node Listing

**Test: List all cluster nodes**
- Given: Cluster with multiple nodes
- Execute: `bink node list`
- Verify: All nodes appear in output
- Verify: Status shown (running/exited/paused)
- Expected: Accurate node listing

**Test: Show node status correctly**
- Given: Cluster with node stopped
- Execute: `bink node list`
- Verify: Stopped node shows correct status
- Expected: Status reflects actual container state

#### 2.2 SSH Access

**Test: Allow SSH to node VM**
- Given: Node created
- Execute: SSH command via bink
- Verify: Command executes successfully
- Verify: Output returned correctly
- Expected: SSH access works (implicitly verifies SSH keys exist, have correct permissions, and are shared across nodes)

---

### 3. API Exposure Tests

**File:** `test/integration/api_test.go`

#### 3.1 API Server Exposure

**Test: Expose API server via SSH tunnel**
- Given: Cluster created
- Execute: `bink api expose`
- Verify: SSH tunnel process running
- Verify: Port 6443 forwarded in container
- Expected: API server accessible from host

**Test: Generate valid kubeconfig**
- Given: API exposed
- Verify: Kubeconfig file created
- Verify: Server URL is https://localhost:6443
- Verify: Valid YAML structure
- Verify: Contains clusters, users, contexts
- Expected: Valid kubeconfig generated

**Test: Handle duplicate expose gracefully**
- Given: API already exposed
- Execute: `bink api expose` again
- Verify: Command succeeds
- Verify: Message indicates tunnel already active
- Expected: Idempotent operation

**Test: Fail if container port not published**
- Given: Custom node without port 6443
- Execute: `bink api expose`
- Verify: Command fails with clear error
- Expected: Helpful error message about missing port

---

### 4. Error Handling & Edge Cases

**File:** `test/integration/cluster_test.go` (additional scenarios)

#### 4.1 Invalid Inputs

**Test: Fail gracefully if images are missing**
- Execute: Create cluster with non-existent image
- Verify: Command fails with clear error
- Verify: Error message mentions missing image
- Expected: User-friendly error handling

**Test: Validate node name format**
- Execute: Add node with invalid names (special chars, spaces, empty)
- Verify: Each fails with validation error
- Expected: Input validation before execution

#### 4.2 Resource Conflicts

**Test: Handle existing node name**
- Given: Node already exists
- Execute: Create node with same name
- Verify: Command fails
- Verify: Error indicates node exists
- Expected: Prevent duplicate nodes

**Test: Handle network conflicts**
- Given: Network already exists
- Execute: Create cluster
- Verify: Network reused or clear error
- Expected: Graceful handling of existing network

---

## Test Helpers & Utilities

### Core Helpers Needed

**`test/integration/helpers/cluster.go`:**
- `GenerateTestClusterName()` - Create unique cluster names
- `BinkCmd(args...)` - Build bink command with args
- `RunCommand(cmd)` - Execute command and return session
- `CreateCluster(name)` - High-level cluster creation
- `AddNode(cluster, node, flags...)` - Add node with options
- `StopCluster(name)` - Stop cluster
- `CleanupCluster(name)` - Full cleanup with --remove-data
- `ExposeAPI(cluster, kubeconfig)` - Expose API server

**`test/integration/helpers/podman.go`:**
- `PodmanCmd(args...)` - Execute podman command
- `PodmanExec(container, cmd)` - Exec in container
- `GetContainer(name)` - Get container info
- `GetContainerID(name)` - Get container ID
- `GetVolume(name)` - Get volume info
- `PodmanStop(container)` - Stop container
- Container inspection utilities

**`test/integration/helpers/ssh.go`:**
- `SSHExec(cluster, node, cmd)` - Execute SSH command in node
- SSH connection helpers
- Key verification utilities

**`test/integration/helpers/cleanup.go`:**
- `CleanupAllTestClusters()` - Remove all test-bink-* clusters
- `RequireCommand(cmd)` - Verify command exists
- `RequireImage(image)` - Verify image exists
- `RequireBink()` - Verify bink binary exists in project root

---

## Execution Strategy

### Makefile Targets

- `test-integration` - Run all integration tests
- `test-integration-quick` - Run quick tests only
- `test-integration-serial` - Run serial tests only
- `test-unit` - Run unit tests
- `test-all` - Run both unit and integration tests

All integration test targets should verify bink binary exists before running.

### Test Execution Time Budget

- **Single comprehensive cluster test** (merged): ~2-3 minutes (vs 8-12 min for 4 separate tests)
- **Cluster stop isolation test** (merged): ~2-3 minutes (vs 4-6 min for 2 separate tests)
- **Multi-node tests**: ~5-7 minutes
- **Full suite serial** (with merged tests): ~8-12 minutes (down from ~15-20 min)
- **Full suite parallel** (with port randomization): ~4-6 minutes! 🚀

**Time Savings:**
- Test merging: ~40-50% reduction
- Port randomization + parallel: ~50% additional reduction
- **Combined: ~70-75% faster** (from 15-20 min to 4-6 min)

### Parallel vs Serial

- **Parallel** (default): Tests run concurrently using auto-assigned ports (`--api-port 0`)
  - No port 6443 conflicts between tests
  - Each cluster gets unique name AND unique port
  - Significant speed improvement (~50% faster)
- **Serial** (optional): Use `-p 1` flag if needed for debugging
  - Example: `go test -v -p 1 ./test/integration/...`

### Retry Strategy

- Use Gomega's `Eventually` for timing-sensitive operations
- SSH connection attempts: 2min timeout, 5s polling
- Node Ready status: 5min timeout, 10s polling
- Pod scheduling: 2min timeout, 5s polling

---

## Open Questions

### 1. Test Image Management

**Question:** How should test images be built and distributed?

**Options:**
- **A)** Build images in CI for each run (slow but always fresh)
- **B)** Cache images in CI (fast but may be stale)
- **C)** Pre-build and push to registry (fastest, requires registry)

**Recommendation:** Start with A, optimize to B with cache invalidation

---

### 2. Resource Cleanup

**Question:** What happens if tests fail mid-execution?

**Options:**
- **A)** Leave resources for debugging - manual cleanup required
- **B)** Always cleanup in `AfterEach` - cleaner but loses debugging
- **C)** Conditional cleanup based on test failure

**Recommendation:** Option C - only cleanup on success, or add `--cleanup-on-failure` flag

---

### 3. External Dependencies

**Question:** How to handle tests requiring external resources (registries, DNS)?

**Options:**
- **A)** Mock all external calls
- **B)** Use local test doubles (local registry, local DNS)
- **C)** Skip tests if dependencies unavailable

**Recommendation:** Option B where possible, Option C for truly external resources

---

### 4. Control-Plane Node Testing

**Question:** Should we test HA control-plane scenarios in every run?

**Options:**
- **A)** Always test multi-control-plane (slower but comprehensive)
- **B)** Test only on main branch (faster PR checks)
- **C)** Make it optional with flag (flexible)

**Recommendation:** Option B - PR tests focus on single control-plane, main branch tests HA

---

### 5. Test Data Management

**Question:** How to manage kubeconfig and other test artifacts?

**Options:**
- **A)** Write to temp directory - clean but harder to debug
- **B)** Write to `test/integration/results/` - easier to inspect
- **C)** Use unique names in project root - visible but clutters

**Recommendation:** Option B with `.gitignore` entry

---

## Next Steps

1. **Review this plan** - Identify gaps, clarify questions
2. **Prioritize test scenarios** - Which tests are critical for MVP?
3. **Answer open questions** - Make decisions on strategy
4. **Set up test structure** - Create directories, suite file, basic helpers
5. **Implement Phase 1** - Cluster creation happy path tests
6. **Iterate and expand** - Add more scenarios based on findings
7. **Integrate with CI** - Automate on every PR

---

## Test Coverage Goals

| Component | Target Coverage | Critical Paths |
|-----------|----------------|----------------|
| Cluster lifecycle | 90% | Create, init, teardown |
| Node operations | 85% | Create, add, list, SSH, role labeling |
| API exposure | 80% | Tunnel, kubeconfig generation |
| DNS management | 75% | DNS entries, resolution |
| Error handling | 70% | Missing images, conflicts, invalid input |

**Overall Target:** 80% code coverage for integration-tested code paths

