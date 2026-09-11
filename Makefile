GO ?= $(shell which go)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)

IMAGE_NAME ?= "ghcr.io/infomaniak/cert-manager-webhook-infomaniak"
IMAGE_TAG ?= "latest"
NAMESPACE ?= "cert-manager-infomaniak"
CHART ?= deploy/infomaniak-webhook

OUT := $(shell pwd)/_out

KUBEBUILDER_VERSION=2.3.1

HELM_FILES := $(shell find deploy/infomaniak-webhook)

test: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl
	TEST_ASSET_ETCD=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd \
	TEST_ASSET_KUBE_APISERVER=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver \
	TEST_ASSET_KUBECTL=_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl \
	$(GO) test -v .

.PHONY: unit-test
unit-test:
	TEST_ASSET_ETCD=/dev/null \
	TEST_ASSET_KUBE_APISERVER=/dev/null \
	TEST_ASSET_KUBECTL=/dev/null \
	$(GO) test -short -v .

.PHONY: unit-test-coverage
unit-test-coverage: | reports/unit
	TEST_ASSET_ETCD=/dev/null \
	TEST_ASSET_KUBE_APISERVER=/dev/null \
	TEST_ASSET_KUBECTL=/dev/null \
	$(GO) test -short -race -covermode=atomic \
		-coverprofile=reports/unit/cover.out .
	$(GO) tool cover -html=reports/unit/cover.out -o reports/unit/coverage.html
	$(GO) run github.com/boumenot/gocover-cobertura@latest \
		< reports/unit/cover.out > reports/unit/cobertura.xml

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz: | _test
	curl -fsSL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH).tar.gz -o $@

_test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/etcd _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kube-apiserver _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)/kubectl: _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH).tar.gz | _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH)
	tar xfO $< kubebuilder_$(KUBEBUILDER_VERSION)_$(OS)_$(ARCH)/bin/$(notdir $@) > $@ && chmod +x $@

.PHONY: clean
clean:
	rm -rf _test $(OUT) reports apiserver.local.config testdata/infomaniak/*.json testdata/infomaniak/*.yaml

.PHONY: build
build:
	docker build -t "$(IMAGE_NAME):$(IMAGE_TAG)" .

.PHONY: deploy
deploy: rendered-manifest.yaml
	kubectl apply -f "$(OUT)/rendered-manifest.yaml"

.PHONY: rendered-manifest.yaml
rendered-manifest.yaml: $(OUT)/rendered-manifest.yaml

$(OUT)/rendered-manifest.yaml: $(HELM_FILES) | $(OUT)
	helm template \
		infomaniak-webhook \
		--namespace $(NAMESPACE) \
		--set image.repository=$(IMAGE_NAME) \
		--set createReleaseNamespace=true \
		$(CHART) > $@

_test $(OUT) reports reports/unit _test/kubebuilder-$(KUBEBUILDER_VERSION)-$(OS)-$(ARCH):
	mkdir -p $@
