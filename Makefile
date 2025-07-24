# Provision and test the kubernetes cluster, this will always teardown old and start new cluster
setup:
	cd scripts && ./setup-cluster.sh

# Kubernetes manifest deployment scripts/setup
SHELL := /bin/bash
NAMESPACE ?= devops-assessment
MANIFEST_DIR := manifests
SERVICE_NAME := nginx-service
DEPLOY_NAME  := nginx-deploy
PORT := 8080
TARGET_PORT := 80

deploy: apply port-forward


apply: _create-namespace
	kubectl apply -n $(NAMESPACE) -f $(MANIFEST_DIR)

delete:
	kubectl delete -n $(NAMESPACE) -f $(MANIFEST_DIR)
	kubectl delete namespace $(NAMESPACE)

port-forward: _wait-for-deployment
	@echo "Forwarding http://localhost:$(PORT) -> $(SERVICE_NAME):$(TARGET_PORT) in namespace $(NAMESPACE)..."
	kubectl port-forward -n $(NAMESPACE) service/$(SERVICE_NAME) $(PORT):$(TARGET_PORT)

_create-namespace:
	kubectl get namespace $(NAMESPACE) >/dev/null 2>&1 || \
	kubectl create namespace $(NAMESPACE)

_wait-for-deployment:
	kubectl wait --for=condition=available --timeout=300s -n $(NAMESPACE) deployment/$(DEPLOY_NAME)

# Nothing fancy, just some quick commands to run/build our go program
GOBUILD := go build
BUILDDIR := build
BINARY_NAME := rollout
MAIN_PATH := ./cmd/main.go

run:
	go run $(MAIN_PATH)

build-all:
	mkdir -p $(BUILDDIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILDDIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILDDIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILDDIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)