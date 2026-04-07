lint:
	@docker run -t --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.11.4 golangci-lint run

test:
	@docker run -t --rm -v $$(pwd):/app -w /app golang:1.25.1-alpine go test ./...
