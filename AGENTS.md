# 🧪 Test Procedure: Cluster Initialization

**Task:** Perform an idempotent startup of the cluster and verify the node status.

---

## 📚 About bootc

This project uses **bootc** for creating bootable container images. For complete documentation, see: https://bootc.dev/bootc/intro.html

### What is bootc?

**bootc** is a tool for performing "Transactional, in-place operating system updates using OCI/Docker container images." It applies Docker's successful layering model to bootable operating systems.

**Key Concepts:**
- Uses standard OCI/Docker containers as a transport and delivery format for base operating system updates
- Container images include a Linux kernel (typically in `/usr/lib/modules`) for booting
- The base userspace doesn't run containerized by default at runtime
- Standard init systems like systemd operate normally as pid1 without an outer wrapper process
- Provides seamless in-place upgrades across system changes
- Builds on the ostree project's proven operating system update infrastructure
- CLI and API are considered stable

**Why bootc for this project:**
This allows us to package and distribute the entire Kubernetes node OS as a container image (`localhost/fedora-bootc-k8s-image:latest`), providing reproducible and transactional cluster deployments.

## Automated Testing

**Integration tests** are now the recommended way to test cluster functionality. Tests use the Ginkgo framework and provide comprehensive validation.

### Quick Start: Run All Tests

```bash
# Run all integration tests (parallel execution with auto-assigned ports)
go test -v ./test/integration/...

# Run tests serially (if needed for debugging)
go test -v -p 1 ./test/integration/...

# Run specific test
go test -v --ginkgo.focus="complete Kubernetes cluster"

# Run with verbose output
go test -v ./test/integration/... -ginkgo.v
```

**Note:** Tests use `--api-port 0` for automatic port assignment, allowing parallel execution without port conflicts.

### Available Test Suites

**1. Complete Cluster Creation Test** (merged test - ~2-3 min)
```bash
go test -v --ginkgo.focus="should create and initialize a complete Kubernetes cluster"
```
Validates: Container creation, Kubernetes initialization, Calico CNI, DNS configuration

**2. Cluster Stop Isolation Test** (~2-3 min)
```bash
go test -v --ginkgo.focus="should isolate cluster stop operations"
```
Validates: Cluster name-based isolation, default vs named cluster separation

**3. Cluster Already Exists Error** (~2 min)
```bash
go test -v --ginkgo.focus="should handle cluster already exists"
```
Validates: Error handling when creating duplicate clusters

### Test Benefits
- ✅ **Comprehensive:** Tests all aspects of cluster creation in one flow
- ✅ **Fast:** Merged tests save ~6-8 minutes vs separate tests
- ✅ **Reliable:** Unique cluster names prevent conflicts
- ✅ **Automated:** Can be run in CI/CD pipelines

---

## Manual Test Procedure

---

## 🔍 Step 1: Pre-Check & Cleanup
Before starting, ensure the environment is clean to avoid naming conflicts.

1. **Check for existing containers:**
   ```bash
   podman ps -a --format "{{.Names}}" | grep -w "k8s-node1"
   ```
2. **Action:** If `k8s-node1` is found, stop the existing cluster:
   ```bash
   ./bink cluster stop
   ```

---

## 🚀 Step 2: Cluster Execution
Execute the start command (images are now provided via container image):

```bash
./bink cluster start
```

**Note:** The `--images-dir` flag has been removed. Images are now mounted from a container image (`localhost/fedora-bootc-k8s-image:latest` by default).

---

## ✅ Step 3: Verification
Verify that the cluster node has initialized successfully and Kubernetes is running.

### 3.1: Container Verification
1. **Command:**
   ```bash
   podman ps
   ```
2. **Success Criteria:**
   - [ ] The output must contain a container named **`k8s-node1`**.
   - [ ] The status of **`k8s-node1`** must be **`Up`**.

### 3.2: Kubernetes Cluster Verification
1. **SSH into the node:**
   ```bash
   ./bink node ssh node1
   ```
2. **Check node status (inside the VM):**
   ```bash
   kubectl get nodes
   ```
3. **Success Criteria:**
   - [ ] Node `node1` is listed
   - [ ] Status is **`Ready`**
   - [ ] Role shows **`control-plane`**

### 3.3: Cluster Components Verification
1. **Check all pods are running (inside the VM):**
   ```bash
   kubectl get pods -A
   ```
2. **Success Criteria:**
   - [ ] All pods in `kube-system` namespace are **`Running`** or **`Completed`**
   - [ ] Calico pods are present and **`Running`**
   - [ ] No pods in `CrashLoopBackOff` or `Error` state

### 3.4: Volume and Storage Verification
Verify that container image mounts and ephemeral storage are working correctly.

1. **Check image volume mount:**
   ```bash
   podman exec k8s-node1 ls -lh /images/
   ```
   **Success Criteria:**
   - [ ] Directory contains `fedora-bootc-k8s.qcow2` base image
   - [ ] File size is approximately 2.5GB

2. **Check cluster-keys volume:**
   ```bash
   podman exec k8s-node1 ls -lh /var/run/cluster/
   ```
   **Success Criteria:**
   - [ ] Contains `cluster.key` (private key)
   - [ ] Contains `cluster.key.pub` (public key)

3. **Check workspace ephemeral storage:**
   ```bash
   podman exec k8s-node1 ls -lh /workspace/
   ```
   **Success Criteria:**
   - [ ] Contains `node1.qcow2` (overlay disk)
   - [ ] Contains `node1-cloud-init.iso` (cloud-init configuration)

4. **Check /opt overlay for CNI (bootc best practice):**
   ```bash
   podman exec k8s-node1 ssh -o StrictHostKeyChecking=no -i /var/run/cluster/cluster.key -p 2222 core@localhost "sudo mount | grep /opt"
   ```
   **Success Criteria:**
   - [ ] Shows `overlay on /opt` with `upperdir=/var/ostree/state-overlays/opt/upper`
   - [ ] CNI binaries present: `sudo ls /opt/cni/bin/`

### 3.5: Alternative Kubernetes Verification (from host)
Instead of SSHing into the VM, you can run kubectl commands directly from the host:

```bash
podman exec k8s-node1 ssh -o StrictHostKeyChecking=no -i /var/run/cluster/cluster.key -p 2222 core@localhost kubectl get nodes
podman exec k8s-node1 ssh -o StrictHostKeyChecking=no -i /var/run/cluster/cluster.key -p 2222 core@localhost kubectl get pods -A
```

### 3.6: Exit SSH Session (if using interactive SSH)
```bash
exit
```

---

## 🚩 Error Handling
* **Stuck Cleanup:** If `./bink cluster stop` fails to remove the container, manually remove it using `podman rm -f k8s-node1` before retrying.
* **Missing Container Image:** If the start command fails with image not found, ensure `localhost/fedora-bootc-k8s-image:latest` exists:
  ```bash
  podman images | grep fedora-bootc-k8s-image
  ```
* **Logs:** If the container is not running, check logs with:
  ```bash
  podman logs k8s-node1
  ```

```

---

## Integration Test Details

The integration tests (`test/integration/cluster_test.go`) provide comprehensive validation:
- **Isolated execution**: Each test uses unique cluster names to prevent conflicts
- **Automatic cleanup**: Resources cleaned up in `AfterEach` hooks
- **Merged tests**: Related validations combined to save time and resources
- **Comprehensive checks**: Validates containers, Kubernetes, CNI, DNS in one flow
- **Error verification**: Tests both success and failure scenarios

### Test Design Philosophy

**Test Merging for Efficiency:**
- Tests validating different aspects of the same command are merged
- Example: Container creation, K8s init, CNI, and DNS are all validated in one test
- **Benefit:** Saves ~6-8 minutes per test suite run (1 cluster creation vs 4)

**When to Use Manual Testing:**
For quick verification during development, you can still manually test:
```bash
./bink cluster start --cluster-name dev-test
podman ps  # Verify container exists
./bink node ssh node1  # SSH into node
kubectl get nodes  # Inside VM
./bink cluster stop --cluster-name dev-test
```

### CI/CD Integration
Integration tests are designed to run in CI pipelines:
- Parallel execution safe (unique cluster names)
- Clean teardown on pass or fail
- ~8-12 minutes for full suite (with merged tests)
- Exit codes indicate success/failure

---

## Changelog

- **2026-04-17**: Implemented API port randomization (`--api-port 0` for auto-assignment)
- **2026-04-17**: Enabled parallel test execution (tests now run in parallel by default)
- **2026-04-17**: Migrated to Ginkgo integration tests (replaced bash test scripts)
- **2026-04-17**: Implemented test merging strategy (saves ~6-8 min per suite run)
- **2026-04-17**: Added cluster stop isolation test (validates cluster name-based separation)
- **2026-04-17**: Updated testing documentation to focus on `go test` workflow
- **2026-04-14**: Fixed Calico CNI installation using `ostree-state-overlay@opt.service` for writable /opt (bootc best practice)
- **2026-04-14**: Migrated to container image for base VM images (removed `--images-dir` flag)
- **2026-04-14**: Added ephemeral storage verification steps (image volume, cluster-keys, workspace)
- **2026-04-14**: Added alternative kubectl verification commands from host
- **2026-04-14**: Fixed container name from `k8s-node` to `k8s-node1` (matches actual implementation)
