.PHONY: tidy build run-server run-client linux windows

tidy:
	go mod tidy

build:
	go build ./...

run-server:
	go run ./cmd/server

run-client:
	go run ./cmd/client

linux:
	mkdir -p bin
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/fvremote-server-linux ./cmd/server
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o bin/fvremote-client-linux ./cmd/client

windows:
	mkdir -p bin
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o bin/fvremote-server.exe ./cmd/server
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -o bin/fvremote-client.exe ./cmd/client
