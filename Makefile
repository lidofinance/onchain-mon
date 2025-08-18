### GO tools
# Makefile
generate-docker:
	docker build -t lidofinance/onchain-mon:stable -f Dockerfile .
.PHONY: generate-docker

.PHONY: tools
tools:
	cd tools && go mod vendor && go mod tidy && go mod verify
	@echo "Installing dev tools into ./bin ..."
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.4.0
	GOBIN=$(PWD)/bin go install github.com/vektra/mockery/v3@v3.5.3
	GOBIN=$(PWD)/bin go install golang.org/x/tools/cmd/goimports@v0.36.0
	GOBIN=$(PWD)/bin go install github.com/atombender/go-jsonschema@v0.20.0
	GOBIN=$(PWD)/bin go install github.com/psampaz/go-mod-outdated@v0.9.0
	GOBIN=$(PWD)/bin go install golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: vendor
vendor:
	go mod tidy && go mod vendor && go mod verify

build:
	go build -o ./bin/service ./cmd/service
.PHONY: build

fmt:
	go fmt ./cmd/... && go fmt ./internal/...

vet:
	go vet ./cmd/... && go vet ./internal/...

imports:
	bin/goimports -local github.com/lidofinance/onchain-mon -w -d $(shell find . -type f -name '*.go'| grep -v "/vendor/\|/.git/\|/tools/")

fix-lint:
	bin/golangci-lint run --config=.golangci.yml --fix ./cmd... ./internal/...

.PHONY: format
format: imports fmt vet

.PHONY: lint
lint:
	bin/golangci-lint run --config=.golangci.yml ./cmd... ./internal/...

outdated:
	@echo "Checking for outdated modules..."
	go list -u -m -json all | ./bin/go-mod-outdated -update -direct
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
	./bin/govulncheck