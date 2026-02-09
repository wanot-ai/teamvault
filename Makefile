.PHONY: build run migrate test clean

DATABASE_URL ?= postgres://teamvault:teamvault_dev@localhost:5432/teamvault?sslmode=disable

build:
	go build -o bin/server ./cmd/server
	go build -o bin/teamvault ./cmd/teamvault

run: build
	DATABASE_URL="$(DATABASE_URL)" \
	JWT_SECRET="dev-jwt-secret-change-in-production" \
	MASTER_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" \
	LISTEN_ADDR=":8443" \
	./bin/server

migrate:
	@echo "Running migrations against $(DATABASE_URL)"
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/server -migrate

test:
	go test -v -race ./...

clean:
	rm -rf bin/

docker-up:
	docker compose up -d postgres

docker-down:
	docker compose down
