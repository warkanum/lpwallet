.PHONY: build test lint deploy version generate

build:
	cp README.md internal/handlers/README.md
	go build -ldflags "-X main.Version=$$(git describe --tags --always 2>/dev/null || echo dev)" -o bin/lpwallet ./cmd/server

test:
	go test ./...

lint:
	golangci-lint run

deploy: build

version:
	@git describe --tags --always 2>/dev/null || echo "dev"

generate:
	relspec convert --from dbml --from-path ./sql/model/database_model.dbml --to gorm --to-path ./internal/models --package models --types stdlib
