# Build Path and Tools
BUILD_PATH ?= .
LOCALBIN ?= $(BUILD_PATH)/operator/bin

GO ?= go
DOCKER ?= docker
CONTROLLER_GEN ?= $(GO) run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.19.0

# Chart paths relative to root
CHART_DIR ?= ./charts/keycloak-operator
CRD_DIR ?= $(CHART_DIR)/crds
CHART_RBAC_DIR ?= $(CHART_DIR)/files

# Paths relative to operator/ for controller-gen outputs
OPERATOR_RBAC_OUT ?= ./config/rbac
CHART_CRD_OUT ?= ../$(CRD_DIR)
CHART_RBAC_OUT ?= ../$(CHART_RBAC_DIR)

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## Clean local binary tools.
	rm -rf $(LOCALBIN)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	cd operator && $(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./api/v1alpha1/...;./internal/controller/..." output:rbac:artifacts:config=$(OPERATOR_RBAC_OUT) output:crd:artifacts:config=$(CHART_CRD_OUT)
	cd operator && $(CONTROLLER_GEN) rbac:roleName=manager-role paths="./api/v1alpha1/...;./internal/controller/..." output:rbac:artifacts:config=$(CHART_RBAC_OUT)

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	cd operator && $(CONTROLLER_GEN) object:headerFile="./hack/boilerplate.go.txt" paths="./api/v1alpha1/...;./internal/controller/..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	cd operator && $(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	cd operator && $(GO) vet ./...

.PHONY: test
test: manifests generate fmt vet ## Run tests.
	cd operator && $(GO) test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	cd operator && $(GO) build -o bin/manager cmd/main.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	cd operator && $(DOCKER) build -t operator:latest .

##@ Tooling

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: ## (Handled dynamically via go run)
