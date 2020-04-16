# Command variables
# Go env variables
GOPATH	= $(shell go env GOPATH)
GOBIN	= $(GOPATH)/bin

# Bin variables
GOLANGCI-LINT = $(GOBIN)/golangci-lint

# Compilation variables
PROJECT_BUILD_SRCS = $(shell git ls-files '*.go' | grep -v '^vendor/')

test-unit:
# Use flag -p 1 to force not to run test in parallel because of
# the presence of different secrets/auths in diffrent tests
	@echo "Launching unit tests..."
	go test -count 1 -p 1 -v -cover ./...

linters:
	$(GOLANGCI-LINT) run ./...

fmt:
	gofmt -s -l -w $(PROJECT_BUILD_SRCS)

deps:
	@echo "Download golangci-lint..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin v1.24.0

.PHONY: fmt deps test-unit linters
