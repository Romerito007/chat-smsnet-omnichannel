.PHONY: build run tidy test vet fmt lint docker-up docker-down

BINARY := bin/chat-backend

build:
	go build -o $(BINARY) .

run:
	go run .

tidy:
	go mod tidy

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

# Boot just the infrastructure (mongodb + redis) for local development.
docker-up:
	docker compose up -d mongodb redis

docker-down:
	docker compose down

# Convenience targets to run a single role locally.
run-api:
	RUN_ROLE=api go run .

run-ws:
	RUN_ROLE=ws go run .

run-worker:
	RUN_ROLE=worker go run .

run-scheduler:
	RUN_ROLE=scheduler go run .
