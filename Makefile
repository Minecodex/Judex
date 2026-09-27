GO ?= go
NPM ?= npm
HELM ?= helm
KUBECTL ?= kubectl
VERSION ?= 0.1.0-dev
IMAGE ?= judex/server
CHART := deploy/helm/judex
HELM_FLAGS := --repository-config deploy/helm/repositories.yaml --repository-cache .cache/helm
ifeq ($(OS),Windows_NT)
EXE := .exe
else
EXE :=
endif

.PHONY: help deps dev api web build build-api build-web test test-go test-web test-e2e check fmt generate helm-lint helm-template-dev helm-template-prod image deploy-dev deploy-prod
help:
	@echo "Judex: deps | api | web | build | test | test-e2e | check | generate | image | helm-lint | deploy-dev | deploy-prod"
deps:
	$(GO) mod download
	$(NPM) ci
generate:
	node api/generate.mjs
	$(GO) tool oapi-codegen -config api/oapi-codegen.yaml internal/gen/api/spec/openapi.yaml
	npx openapi-typescript api/openapi.yaml -o web/src/lib/api/schema.d.ts
	$(GO) build ./internal/gen/...
dev: api
api:
	$(GO) run ./cmd/judex-server
web:
	$(NPM) run dev --workspace @judex/web
build: build-api build-web
build-api:
	$(GO) build -trimpath -ldflags "-X github.com/kakj-go/Judex/internal/version.Version=$(VERSION)" -o bin/judex-server$(EXE) ./cmd/judex-server
	$(GO) build -trimpath -o bin/judex$(EXE) ./cmd/judex
build-web:
	$(NPM) run build --workspace @judex/web
test: test-go test-web
test-go:
	$(GO) test ./...
test-web:
	$(NPM) run test:web
test-e2e: build-web
	$(NPM) run test:e2e
check: test build helm-lint test-e2e
fmt:
	$(GO) fmt ./...
helm-lint:
	$(HELM) $(HELM_FLAGS) lint $(CHART) -f $(CHART)/values-dev.yaml
	$(HELM) $(HELM_FLAGS) lint $(CHART) -f $(CHART)/values-prod.yaml
helm-template-dev:
	$(HELM) $(HELM_FLAGS) template judex $(CHART) -n judex-dev -f $(CHART)/values-dev.yaml
helm-template-prod:
	$(HELM) $(HELM_FLAGS) template judex $(CHART) -n judex-prod -f $(CHART)/values-prod.yaml
image:
	docker build -f deploy/docker/server.Dockerfile --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .
deploy-dev:
	$(HELM) $(HELM_FLAGS) upgrade --install judex $(CHART) -n judex-dev --create-namespace -f $(CHART)/values-dev.yaml --set image.repository=$(IMAGE) --set image.tag=$(VERSION) --wait --timeout 10m
deploy-prod:
	$(HELM) $(HELM_FLAGS) upgrade --install judex $(CHART) -n judex-prod --create-namespace -f $(CHART)/values-prod.yaml --set image.repository=$(IMAGE) --set image.tag=$(VERSION) --wait --timeout 10m
