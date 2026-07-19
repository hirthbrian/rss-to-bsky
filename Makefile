.PHONY: build vet test check run docker-up docker-logs

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

check: build vet test

run:
	set -a; . ./.env; set +a; go run ./cmd/rss-to-bsky

docker-up:
	docker compose up -d --build

docker-logs:
	docker compose logs -f
