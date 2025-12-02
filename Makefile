GOFILES := $(shell find . -name '*.go' -not -path './vendor/*')

.PHONY: fmt
fmt:
	@gofmt -w $(GOFILES)
	@goimports -w $(GOFILES)

.PHONY: lint
lint:
	@golangci-lint run ./...

.PHONY: test
test:
	@go test ./...

.PHONY: all
all: fmt lint test
