# 🧪 Test Procedure: Cluster Initialization

**Task:** Perform an idempotent startup of the cluster and verify the node status.

## Automated Test Script

For automated testing, use the provided test script:

```bash
cd bink
./test-cluster.sh
```

The script follows all steps below and provides colored output for easy verification.

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

## Test Script Details

The automated test script (`test-cluster.sh`) includes:
- **Color-coded output**: Green for success, yellow for warnings, red for failures
- **Automatic cleanup**: Detects and stops existing containers before starting
- **Error handling**: Provides detailed logs if tests fail
- **Verification**: Checks container existence and running status
- **Summary**: Shows cluster details and next steps

### Pro-Tip for the Agent:
If you are using a more advanced AI agent (like one with a bash tool), you can instruct it to use this logic: `if podman ps -a | grep -q k8s-node1; then ./bink cluster stop; fi`. This makes the "Cleanup" step fully autonomous!

---

## Changelog

- **2026-04-14**: Fixed Calico CNI installation using `ostree-state-overlay@opt.service` for writable /opt (bootc best practice)
- **2026-04-14**: Migrated to container image for base VM images (removed `--images-dir` flag)
- **2026-04-14**: Added ephemeral storage verification steps (image volume, cluster-keys, workspace)
- **2026-04-14**: Added alternative kubectl verification commands from host
- **2026-04-14**: Fixed container name from `k8s-node` to `k8s-node1` (matches actual implementation)
- **2026-04-14**: Added automated test script (`test-cluster.sh`) for easier testing
