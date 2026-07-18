# KubeSpaces operator
#
# Requires Go 1.26+. controller-gen is invoked via `go run` (pinned in go.mod
# through the tool dependency), so no separate install step is needed.

CONTROLLER_GEN = go run sigs.k8s.io/controller-tools/cmd/controller-gen

.PHONY: all
all: build

##@ Development

.PHONY: generate
generate: ## Generate zz_generated.deepcopy.go.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..."

.PHONY: manifests
manifests: ## Generate CRD and RBAC manifests into config/.
	$(CONTROLLER_GEN) crd rbac:roleName=kubespaces-operator paths="./..." \
		output:crd:artifacts:config=config/crd \
		output:rbac:artifacts:config=config/rbac

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: ## Run unit tests.
	go test ./... -race -count=1

##@ Build

.PHONY: build
build: ## Build the operator binary.
	go build -o bin/operator ./cmd

.PHONY: run
run: ## Run the operator against the current kubeconfig context.
	go run ./cmd

IMG ?= ghcr.io/kubespaces-io/operator:dev

.PHONY: docker-build
docker-build: ## Build the container image.
	docker build -t $(IMG) .

##@ Deployment

.PHONY: install
install: ## Install the Tenant CRD into the current cluster.
	kubectl apply -f config/crd/

.PHONY: uninstall
uninstall: ## Remove the Tenant CRD (deletes all Tenants!).
	kubectl delete -f config/crd/

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
