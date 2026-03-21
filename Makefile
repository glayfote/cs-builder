.PHONY: up shell run build test tidy down

up:
	docker compose up -d --build

down:
	docker compose down

shell:
	docker compose run --rm app bash

run:
	docker compose run --rm app go run .

build:
	docker compose run --rm app go build -o bin/buildtool .

test:
	docker compose run --rm app go test ./...

tidy:
	docker compose run --rm app go mod tidy
