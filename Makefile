IMG ?= clustergate:latest

# Tool versions
CONTROLLER_TOOLS_VERSION ?= v0.17.0
ENVTEST_K8S_VERSION ?= 1.33.0

# Local bin directory for tool binaries
LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: all
all: build

##@ Development

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: fmt vet envtest ## Run all tests (unit + integration).
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./... -coverprofile cover.out

.PHONY: test-unit
test-unit: fmt vet ## Run unit tests only (no envtest).
	go test $$(go list ./... | grep -v test/integration) -coverprofile cover.out

.PHONY: test-integration
test-integration: fmt vet envtest ## Run integration tests only.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
		go test ./test/integration/... -coverprofile cover-integration.out -v

.PHONY: lint
lint: ## Run golangci-lint.
	golangci-lint run ./...

##@ Code Generation

.PHONY: generate
generate: controller-gen ## Generate DeepCopy methods.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: manifests
manifests: controller-gen ## Generate CRD and RBAC manifests.
	$(CONTROLLER_GEN) rbac:roleName=clustergate-controller crd:allowDangerousTypes=true \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases \
		output:rbac:dir=config/rbac

##@ Build

.PHONY: build
build: generate fmt vet ## Build the manager binary.
	CGO_ENABLED=0 go build -o bin/manager ./cmd/manager

.PHONY: build-cli
build-cli: fmt vet ## Build the clustergate CLI binary.
	CGO_ENABLED=0 go build -o bin/clustergate ./cmd/clustergate

.PHONY: build-all
build-all: build build-cli ## Build both the manager and CLI binaries.

.PHONY: run
run: generate fmt vet ## Run the controller from your host.
	go run ./cmd/manager

.PHONY: docker-build
docker-build: ## Build the Docker image.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the Docker image.
	docker push $(IMG)

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the cluster.
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the cluster.
	kubectl delete -f config/crd/bases/

.PHONY: deploy
deploy: manifests ## Deploy controller to the cluster.
	kubectl apply -f config/manager/namespace.yaml
	kubectl apply -f config/crd/bases/
	kubectl apply -f config/rbac/service_account.yaml
	kubectl apply -f config/rbac/role.yaml
	kubectl apply -f config/rbac/role_binding.yaml
	kubectl apply -f config/rbac/leader_election_role.yaml
	kubectl apply -f config/rbac/leader_election_role_binding.yaml
	kubectl apply -f config/manager/manager.yaml
	kubectl apply -f config/manager/pdb.yaml

.PHONY: undeploy
undeploy: ## Undeploy controller from the cluster.
	kubectl delete -f config/manager/manager.yaml || true
	kubectl delete -f config/manager/pdb.yaml || true
	kubectl delete -f config/rbac/leader_election_role_binding.yaml || true
	kubectl delete -f config/rbac/leader_election_role.yaml || true
	kubectl delete -f config/rbac/role_binding.yaml || true
	kubectl delete -f config/rbac/role.yaml || true
	kubectl delete -f config/rbac/service_account.yaml || true
	kubectl delete -f config/crd/bases/ || true
	kubectl delete -f config/manager/namespace.yaml || true

.PHONY: sample
sample: ## Apply sample CRs.
	kubectl apply -f config/samples/clusterreadiness_v1alpha1.yaml
	kubectl apply -f config/samples/gatecheck_v1alpha1.yaml
	kubectl apply -f config/samples/gateprofile_v1alpha1.yaml

##@ Tool Dependencies

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	@test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	@test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

##@ Help

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
