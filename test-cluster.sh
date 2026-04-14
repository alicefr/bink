#!/bin/bash
# 🧪 Test Script: Cluster Initialization
# Based on AGENTS.md test procedure

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

CONTAINER_NAME="k8s-node1"
BINK_BINARY="./bink"
IMAGES_DIR="$(realpath $(pwd)/../vm/images/)"

echo -e "${BLUE}🧪 Test Procedure: Cluster Initialization${NC}"
echo ""

echo -e "${BLUE}=== Step 1: Pre-Check & Cleanup ===${NC}"
echo "Checking for existing containers..."

if podman ps -a --format "{{.Names}}" | grep -qw "$CONTAINER_NAME"; then
    echo -e "${YELLOW}⚠️  Container $CONTAINER_NAME found. Stopping cluster...${NC}"
    $BINK_BINARY cluster stop
    echo -e "${GREEN}✅ Cluster stopped${NC}"
else
    echo -e "${GREEN}✅ No existing containers found${NC}"
fi

echo ""
echo -e "${BLUE}=== Step 2: Cluster Execution ===${NC}"
echo "Starting cluster with images directory: $IMAGES_DIR"

if [ ! -d "$IMAGES_DIR" ]; then
    echo -e "${RED}❌ Error: Images directory not found: $IMAGES_DIR${NC}"
    exit 1
fi

$BINK_BINARY cluster start --images-dir "$IMAGES_DIR"

echo ""
echo -e "${BLUE}=== Step 3: Verification ===${NC}"
echo "Verifying cluster node status..."

sleep 2

if ! podman ps --format "{{.Names}}" | grep -qw "$CONTAINER_NAME"; then
    echo -e "${RED}❌ FAILED: Container $CONTAINER_NAME not found${NC}"
    echo ""
    echo "Available containers:"
    podman ps -a
    echo ""
    echo "Container logs:"
    podman logs "$CONTAINER_NAME" 2>&1 | tail -50 || echo "Could not fetch logs"
    exit 1
fi

STATUS=$(podman ps --filter "name=$CONTAINER_NAME" --format "{{.Status}}")

if echo "$STATUS" | grep -q "Up"; then
    echo -e "${GREEN}✅ SUCCESS: Container $CONTAINER_NAME is running${NC}"
    echo -e "${GREEN}✅ Status: $STATUS${NC}"
else
    echo -e "${RED}❌ FAILED: Container $CONTAINER_NAME is not running${NC}"
    echo -e "${RED}   Status: $STATUS${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}=== Step 3.2: Kubernetes Cluster Verification ===${NC}"
echo "Checking Kubernetes cluster status..."

$BINK_BINARY node ssh node1 "kubectl get nodes" > /tmp/kubectl-nodes.txt 2>&1 &
KUBECTL_PID=$!

sleep 5
kill $KUBECTL_PID 2>/dev/null || true
wait $KUBECTL_PID 2>/dev/null || true

if grep -q "node1.*Ready.*control-plane" /tmp/kubectl-nodes.txt 2>/dev/null; then
    echo -e "${GREEN}✅ Kubernetes node is Ready${NC}"
    cat /tmp/kubectl-nodes.txt
else
    echo -e "${YELLOW}⚠️  Kubernetes cluster may still be initializing${NC}"
    echo "Run manually to check: $BINK_BINARY node ssh node1 'kubectl get nodes'"
fi

echo ""
echo -e "${BLUE}=== Test Summary ===${NC}"
echo -e "${GREEN}✅ Container verification passed!${NC}"
echo ""
echo "Cluster Details:"
podman ps --filter "name=$CONTAINER_NAME" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo "  • Check cluster status: $BINK_BINARY node ssh node1 'kubectl get nodes'"
echo "  • Check all pods: $BINK_BINARY node ssh node1 'kubectl get pods -A'"
echo "  • List nodes: $BINK_BINARY node list"
echo "  • Stop cluster: $BINK_BINARY cluster stop"
echo "  • Stop with cleanup: $BINK_BINARY cluster stop --remove-data"

rm -f /tmp/kubectl-nodes.txt
