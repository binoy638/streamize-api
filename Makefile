.PHONY: api-dev api-test api-tidy web-dev web-build web-test compose-config

api-dev:
	cd apps/api && go run ./cmd/api

api-test:
	cd apps/api && go test ./...

api-tidy:
	cd apps/api && go mod tidy

web-dev:
	cd apps/web && npm run dev

web-build:
	cd apps/web && npm run build

web-test:
	cd apps/web && npm run test

compose-config:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example config
