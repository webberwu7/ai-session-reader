.PHONY: build test check
build:
	go build -o bin/ai-session ./cmd/ai-session
test:
	go test -race ./...
check: test
	go vet ./...
