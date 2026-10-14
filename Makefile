.PHONY: build install test

# Binaries land in bin/ (build) or $GOBIN, default $(go env GOPATH)/bin (install).
build:
	go build -o bin/ ./scripts/...

install:
	go install ./scripts/...

test:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go test ./...
	go run ./scripts/run_bot -seed 1 | grep -q 'balance=1820.*done=true'
