CHART        := charts/kubespaces
RELEASE      := kubespaces
NAMESPACE    := kubespaces
KIND_CLUSTER := kubespaces-dev

.PHONY: lint template install uninstall kind-up kind-down kind-install crds sync-crds

lint: sync-crds
	helm lint $(CHART)
	helm lint $(CHART) -f examples/values-production.yaml

# Sync the operator-generated CRD into the chart. The operator config
# (operator/config/crd) is the single source of truth; the chart ships a copy
# under files/ that templates/operator/crd.yaml renders. Run after
# `make -C operator manifests`.
sync-crds:
	cp operator/config/crd/kubespaces.io_tenants.yaml $(CHART)/files/crd-tenants.yaml

template:
	helm template $(RELEASE) $(CHART) --namespace $(NAMESPACE)

install:
	helm upgrade --install $(RELEASE) $(CHART) \
		--namespace $(NAMESPACE) --create-namespace

uninstall:
	helm uninstall $(RELEASE) --namespace $(NAMESPACE)

crds:
	kubectl apply -f operator/config/crd/

kind-up:
	kind create cluster --name $(KIND_CLUSTER)

kind-down:
	kind delete cluster --name $(KIND_CLUSTER)

kind-install:
	helm upgrade --install $(RELEASE) $(CHART) \
		--namespace $(NAMESPACE) --create-namespace \
		-f examples/values-kind.yaml \
		--kube-context kind-$(KIND_CLUSTER)
