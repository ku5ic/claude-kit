.PHONY: check lint test build deploy

check: lint test

lint:
	golangci-lint run ./...

test:
	go test ./...

build:
	go build -o bin/app ./cmd/app

deploy: build
	./scripts/deploy.sh
