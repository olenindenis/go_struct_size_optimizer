lint:
	@docker run -t --rm -v $$(pwd):/app -w /app golangci/golangci-lint:v2.11.4 golangci-lint run

test:
	go test ./...
