.PHONY: run test build sqlc docker-up docker-down migrate-up migrate-down seed

export PATH := $(PATH):$(HOME)/go/bin

DB_URL ?= postgres://mergiate:mergiate_password@localhost:5434/mergiate_core?sslmode=disable
EMAIL ?= admin@akordium.id
PASSWORD ?= Secret123!
TENANT ?= akordium
ORG ?= Akordium Main Org

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test -v ./...

sqlc:
	sqlc generate

docker-up:
	docker compose up -d

docker-down:
	docker compose down

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

seed:
	go run ./cmd/seed -email="$(EMAIL)" -password="$(PASSWORD)" -tenant="$(TENANT)" -org="$(ORG)"
