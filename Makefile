GO ?= go
BINARY ?= aibar
PACKAGES := ./cmd/... ./internal/...

.PHONY: check fmt lint vet test race build run tidy

## check: format, lint, and test — the pre-submit gate
check: fmt lint test

## fmt: apply gofmt and the wsl_5 blank-line convention
fmt:
	gofmt -w ./cmd ./internal
	wsl -fix $(PACKAGES)

## lint: run go vet and the repository linters (includes depguard layering rules)
lint: vet
	golangci-lint run ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## test: run all package tests
test:
	$(GO) test ./...

## race: run all tests under the race detector
race:
	$(GO) test -race ./...

## build: compile the daemon binary
build:
	$(GO) build -o $(BINARY) ./cmd/aibar

## run: run the daemon against the real ~/.codex / ~/.claude sources
run:
	$(GO) run ./cmd/aibar daemon

## tidy: prune and verify module requirements
tidy:
	$(GO) mod tidy
