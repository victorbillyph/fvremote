.PHONY: tidy build test run linux windows e2e

tidy:
	go mod tidy

build:
	go build ./...

test:
	go test ./...

# Teste ponta-a-ponta real (sobe Tor e conecta via onion). Requer rede.
e2e:
	go test -tags e2e -run TestEndToEnd -v -timeout 300s ./internal/e2e/

run:
	go run ./cmd/fvremote

linux:
	mkdir -p bin
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/fvremote-linux ./cmd/fvremote

windows:
	mkdir -p bin
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -trimpath -o bin/fvremote.exe ./cmd/fvremote
