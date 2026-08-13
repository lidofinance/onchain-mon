### GO tools
# Makefile
generate-docker:
	docker build -t lidofinance/onchain-mon:stable -f Dockerfile .
.PHONY: generate-docker

.PHONY: tools
tools:
	@echo "Installing dev tools into ./bin ..."
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
	GOBIN=$(PWD)/bin go install github.com/vektra/mockery/v3@v3.7.3
	GOBIN=$(PWD)/bin go install golang.org/x/tools/cmd/goimports@v0.48.0
	GOBIN=$(PWD)/bin go install github.com/atombender/go-jsonschema@v0.24.1
	GOBIN=$(PWD)/bin go install github.com/psampaz/go-mod-outdated@v0.9.0
	GOBIN=$(PWD)/bin go install golang.org/x/vuln/cmd/govulncheck@v1.6.0

.PHONY: vendor
vendor:
	go mod tidy && go mod vendor && go mod verify

build:
	go build -o ./bin/service ./cmd/service
.PHONY: build

fmt:
	bin/golangci-lint fmt --config=.golangci.yml ./cmd/... ./internal/...

vet:
	go vet ./cmd/... && go vet ./internal/...

imports:
	bin/goimports -local github.com/lidofinance/onchain-mon -w $(shell find ./cmd ./internal -type f -name '*.go')

fix-lint:
	bin/golangci-lint run --config=.golangci.yml --fix ./cmd... ./internal/...

.PHONY: test
test:
	go test ./cmd/... ./internal/...

# Tests behind the `live` tag read the repo-root .env / notification.yaml and hit
# real RPC and messaging APIs — they can post messages to real channels.
.PHONY: test-live
test-live:
	go test -tags=live ./cmd/... ./internal/...

.PHONY: fmt vet imports format
format: imports fmt vet

.PHONY: lint
lint:
	bin/golangci-lint run --config=.golangci.yml ./cmd... ./internal/...

outdated:
	@echo "Checking for outdated modules..."
	go list -u -m -json -mod=mod all | ./bin/go-mod-outdated -update -direct
.PHONY: outdated

generate-databus-objects:
	for file in ./brief/databus/*.dto.json; do \
		base_name=$$(basename $$file .dto.json); \
		bin/go-jsonschema -p databus -o generated/databus/$$base_name.dto.go $$file; \
	done
.PHONY: generate-databus-objects

.PHONY: vulncheck
vulncheck:
	@echo "Running govulncheck..."
	./bin/govulncheck -show verbose ./...
