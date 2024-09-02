//go:build tools
// +build tools

//go:generate bash -c "go build -ldflags \"-X 'main.version=$(go list -m -f '{{.Version}}' github.com/golangci/golangci-lint)' -X 'main.commit=test' -X 'main.date=test'\" -o ../bin/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint"
//go:generate go build -o ../bin/mockery github.com/vektra/mockery/v2
//go:generate go build -o ../bin/goimports golang.org/x/tools/cmd/goimports
//go:generate go build -o ../bin/swagger  github.com/go-swagger/go-swagger/cmd/swagger
//go:generate go build -o ../bin/jsonschema github.com/atombender/go-jsonschema

// Package tools contains go:generate commands for all project tools with versions stored in local go.mod file
// See https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
package tools

import (
	_ "github.com/atombender/go-jsonschema"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/vektra/mockery/v2"
	_ "golang.org/x/tools/cmd/goimports"
)
