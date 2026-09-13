BIN := bin/observatory

.PHONY: all build run test cover vet lint fmt install clean

all: lint test build

build:
	go build -o $(BIN) ./cmd/observatory

run:
	go run ./cmd/observatory

test:
	go test ./...

cover:
	go test -cover ./...

vet:
	go vet ./...

lint: vet
	test -z "$$(gofmt -l .)"
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

fmt:
	go fmt ./...

install:
	go install ./cmd/observatory

clean:
	rm -rf bin
