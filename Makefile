GOFILES := $(shell find . -name '*.go' -not -path './vendor/*')
UNAME_S := $(shell uname -s)

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

.PHONY: build
ifeq ($(UNAME_S),Darwin)
build:
	@go build -o camsnap -ldflags '-linkmode external -extldflags "-Wl,-sectcreate,__TEXT,__info_plist,$(CURDIR)/internal/avf/Info.plist"' ./cmd/camsnap
	@codesign --force -s "$${CAMSNAP_CODESIGN_IDENTITY:--}" camsnap
	@codesign --verify --verbose camsnap
	@echo "Signed camsnap with identity: $${CAMSNAP_CODESIGN_IDENTITY:--}"
else
build:
	@go build -o camsnap ./cmd/camsnap
endif
