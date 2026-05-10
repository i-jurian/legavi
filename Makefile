.PHONY: dev dev-down test test-be test-fe lint lint-be lint-fe

COMPOSE_FILE := deploy/docker/compose.yaml

dev:
	docker compose -f $(COMPOSE_FILE) up -d

dev-down:
	docker compose -f $(COMPOSE_FILE) down

test: test-be test-fe

test-be:
	cd backend && go test ./...

test-fe:
	cd frontend && npm test

lint: lint-be lint-fe

lint-be:
	cd backend && golangci-lint run

lint-fe:
	cd frontend && npm run lint