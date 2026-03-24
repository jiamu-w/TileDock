APP_NAME=panel

.PHONY: run test build tidy

run:
	go run ./cmd/server

test:
	go test ./...

build:
	go build -o bin/$(APP_NAME) ./cmd/server

tidy:
	go mod tidy
