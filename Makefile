SHELL := /bin/bash
NAMESPACE ?= devops-assessment
MANIFEST_DIR := manifests
SERVICE_NAME := nginx-service
PORT := 8080
TARGET_PORT := 80

.PHONY: all apply port-forward create-namespace

deploy: apply port-forward

apply: _create-namespace
	kubectl apply -n $(NAMESPACE) -f $(MANIFEST_DIR)

port-forward:
	@echo "Forwarding http://localhost:$(PORT) -> $(SERVICE_NAME):$(TARGET_PORT) in namespace $(NAMESPACE)..."
	kubectl port-forward -n $(NAMESPACE) service/$(SERVICE_NAME) $(PORT):$(TARGET_PORT)

_create-namespace:
	kubectl get namespace $(NAMESPACE) >/dev/null 2>&1 || \
	kubectl create namespace $(NAMESPACE)