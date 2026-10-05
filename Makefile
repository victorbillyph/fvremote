.PHONY: tidy build test run linux windows e2e

tidy:
	go mod tidy

# Compila o app (Wails embute o frontend em frontend/dist)
build:
	wails build -s -skipbindings -m -nosyncgomod -tags webkit2_41

linux: build

# No Windows, execute nativamente (o Wails não faz cross-compile):
windows:
	wails build -s -skipbindings -m -nosyncgomod

run:
	go run -tags "desktop,production,webkit2_41" .

test:
	go test ./internal/identity/ ./internal/hub/

# Teste ponta-a-ponta real (sobe Tor e conecta via onion). Requer rede.
e2e:
	go test -tags e2e -run TestEndToEnd -v -timeout 300s ./internal/e2e/
