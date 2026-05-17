.PHONY: api-dev api-test api-tidy compose-config

api-dev:
	cd apps/api && go run ./cmd/api

api-test:
	cd apps/api && go test ./...

api-tidy:
	cd apps/api && go mod tidy

compose-config:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config
