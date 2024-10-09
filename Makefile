### GO tools
# Makefile
generate-docker:
	docker build -t lidofinance/onchain-mon:stable -f Dockerfile .
.PHONY: generate-docker

.PHONY: tools
tools:
	cd tools && go mod vendor && go mod tidy && go mod verify && go generate -tags tools

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
format: imports fmt vet fix-lint

.PHONY: lint
lint:
	bin/golangci-lint run --config=.golangci.yml ./cmd... ./internal/...

outdated-deps:
	go list -u -m -json -mod=readonly all
.PHONY: outdated-deps

generate-databus-objects:
	for file in ./brief/databus/*.dto.json; do \
		base_name=$$(basename $$file .dto.json); \
		bin/jsonschema -p databus -o generated/databus/$$base_name.dto.go $$file; \
	done
.PHONY: generate-databus-objects