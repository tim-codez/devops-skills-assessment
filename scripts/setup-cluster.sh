#!/bin/bash

# Script:  setup-cluster.sh
# Purpose: Provision and manage the lifecycle of a Kubernetes cluster utilizing Kind (Kubernetes-in-docker).

set -e

CLUSTER_NAME="devops-assessment"

echo "Setting up local Kubernetes cluster..."

# Check for binaries required to setup and manage cluster (docker/kubectl/kind). The script will install kind and kubectl automatically, but I left out docker as it's a bit more involved.
if ! command -v docker &> /dev/null; then
    echo -e "docker needs to be installed before continuing.\nPlease install docker: https://docs.docker.com/engine/install/"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo "Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/kubectl

    if ! command -v kubectl &> /dev/null; then
        echo "Failed to install kubectl"
        exit 1
    fi
    echo "kubectl installed successfully"
fi

if ! command -v kind &> /dev/null; then
    echo "Installing kind..."
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    sudo mv ./kind /usr/local/bin/kind

    if ! command -v kind &> /dev/null; then
        echo "Failed to install kind"
        exit 1
    fi
    echo "kind installed successfully"
fi


# Clean up existing cluster if it exists, 
if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Removing existing cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name ${CLUSTER_NAME}
    rm kind-cluster.yaml
    echo "Existing cluster removed"
fi

# Create a simple kind cluster resource, nothing special here
cat > kind-cluster.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
EOF

 # Create a kubernetes cluster using kind
echo "Creating cluster..."
if kind create cluster --config kind-cluster.yaml; then
    echo "Cluster created successfully!"
else
    echo "Failed to create cluster"
    exit 1
fi

# Sleep just a few seconds, sometimes you can't access the cluster immedidiately after setting up kind
sleep 3

# Run through a couple verification steps to ensure cluster has been installed and that it is accessible and operable.
echo "Verifying cluster access..."
if kubectl cluster-info &> /dev/null; then
    echo "kubectl can access the cluster"
    kubectl cluster-info
else
    echo "Cannot access cluster with kubectl"
    exit 1
fi

echo "Waiting for nodes to be ready..."
if kubectl wait --for=condition=Ready nodes --all --timeout=60s &> /dev/null; then
    echo "All nodes are ready"
    kubectl get nodes
else
    echo "Nodes did not become ready in time, you might want to try running 'kubectl get nodes' to investigate further"
    exit 1
fi