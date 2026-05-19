BIN_NAME  := opcfs
CMD_PATH  := ./cmd/server
BIN_DIR   := bin

.PHONY: build run test lint docker-build tidy

## build: compile the server binary to bin/opcfs
build:
	go build -o $(BIN_DIR)/$(BIN_NAME) $(CMD_PATH)

## run: run the server directly via go run
run:
	go run $(CMD_PATH)

## test: run all tests with race detector
test:
	go test ./... -race -count=1

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## docker-build: build the production Docker image
docker-build:
	docker build -f deploy/docker/Dockerfile -t $(BIN_NAME):latest .

## tidy: tidy and verify go.mod / go.sum
tidy:
	go mod tidy
	go mod verify

## help: print this help message
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ": "}; {printf "  %-16s %s\n", $$2, $$3}'
