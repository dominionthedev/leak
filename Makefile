.PHONY: build demo tidy test fmt check

build:
	go build ./...

demo:
	cd examples/basic && go run .

tidy:
	go mod tidy

test:
	go test ./... -race

fmt:
	gofmt -w .

check:
	./scripts/check.sh
