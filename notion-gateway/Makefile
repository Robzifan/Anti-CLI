.PHONY: fmt lint test test-race test-fuzz test-integration build docker smoke security
fmt:
	@command -v gofmt >/dev/null || (echo "gofmt not installed"; exit 127)
	@test -z "$$(gofmt -l .)" || (gofmt -d .; exit 1)
lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null; then golangci-lint run; else echo "golangci-lint not installed; go vet completed"; fi
test:
	go test ./...
test-race:
	go test -race ./...
test-fuzz:
	go test -run=^$$ -fuzz=FuzzLoad -fuzztime=10s ./internal/notion/fixtures
	go test -run=^$$ -fuzz=FuzzStreamDecoder -fuzztime=10s ./internal/notion/protocol
	go test -run=^$$ -fuzz=FuzzParse -fuzztime=10s ./internal/notion/discovery/picker
test-integration:
	go test -count=1 ./internal/api/...
build:
	go build ./cmd/gateway ./cmd/notion-codex
	dir="$$(mktemp -d)"; GOOS=linux GOARCH=amd64 go build -o "$$dir/gateway-linux-amd64" ./cmd/gateway; GOOS=darwin GOARCH=arm64 go build -o "$$dir/gateway-darwin-arm64" ./cmd/gateway; GOOS=windows GOARCH=amd64 go build -o "$$dir/gateway-windows-amd64.exe" ./cmd/gateway; rm -rf "$$dir"
docker:
	docker build -t notioncodex-gateway:dev .
smoke: docker
	docker run --rm -d --name notioncodex-smoke -p 127.0.0.1:18787:8787 notioncodex-gateway:dev; trap 'docker rm -f notioncodex-smoke >/dev/null 2>&1 || true' EXIT; sleep 2; curl --fail http://127.0.0.1:18787/healthz
security:
	python3 scripts/check-secrets.py
