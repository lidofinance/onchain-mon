### GO tools
.PHONY: tools
tools:
	cd tools && go mod vendor && go mod tidy && go mod verify && go generate -tags tools

.PHONY: vendor
vendor:
	go mod tidy && go mod vendor &&  go mod verify

build:
	go build -o ./bin/service ./cmd/service
.PHONY: build

fmt:
	go fmt ./cmd/... && go fmt ./internal/...

vet:
	go vet ./cmd/... && go vet ./internal/...

imports:
	bin/goimports -local github.com/lidofinance/finding-forwarder -w -d $(shell find . -type f -name '*.go'| grep -v "/vendor/\|/.git/\|/tools/")

fix-lint:
	bin/golangci-lint run --config=.golangci.yml --fix ./cmd... ./internal/...

lint:
	bin/golangci-lint run --config=.golangci.yml ./cmd... ./internal/...

.PHONY: full-lint
full-lint: imports fmt vet lint

.PHONY: swagger-gen
swagger-gen:
	rm -rf generated/forta && \
	bin/swagger generate server \
		-f ./brief/forta-webhook/swagger.yml \
		-m generated/forta/models \
		--exclude-main \
		--skip-support \
		--skip-operations
