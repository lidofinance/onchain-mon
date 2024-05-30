### GO tools
tools:
	cd tools && go mod vendor && go mod tidy && go mod verify && go generate -tags tools
.PHONY: tools

vendor:
	go mod tidy && go mod vendor &&  go mod verify
.PHONY: vendor

build:
	go build -o ./bin/service ./cmd/service
.PHONY: build

fmt:
	go fmt ./cmd/... && go fmt ./internal/...

vet:
	go vet ./cmd/... && go vet ./internal/...

imports:
	bin/goimports -local github.com/lidofinance/finding-forwarder -w -d $(shell find . -type f -name '*.go'| grep -v "/vendor/\|/.git/\|/tools/")

lint:
	bin/golangci-lint run --config=.golangci.yml --fix ./cmd... ./internal/...

full-lint: imports fmt vet lint
.PHONY: full-lint

full-lint: imports fmt vet lint
.PHONY: full-lint

swagger-gen:
	rm -rf generated/forta && \
	bin/swagger generate server \
		-f ./brief/forta-webhook/swagger.yml \
		-m generated/forta/models \
		--exclude-main \
		--skip-support \
		--skip-operations

.PHONY: swagger-gen