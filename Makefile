.PHONY: build run test vet lint docker clean

BIN := pgadmin-go
PKG := ./...

build:
	go build -o bin/$(BIN) ./cmd/pgadmin-server

run: build
	./bin/$(BIN) -config config/config.yaml

test:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -n 20

vet:
	go vet ./...

lint:
	golangci-lint run ./... || true

docker:
	docker build -t pgadmin-go:latest .

clean:
	rm -rf bin/ coverage.out /tmp/pgadmin-go

# Integration against ephemeral PG (requires Docker)
integration:
	docker compose -f docker-compose.test.yml up --abort-on-container-exit --exit-code-from tests

# Generate mocks / update deps
tidy:
	go mod tidy
