.PHONY: migrate-create migrate-up migrate-down migrate-status migrate-reset test test-integration test-db-up test-db-down

MIGRATIONS=migrations

migrate-create:
	goose -dir $(MIGRATIONS) create $(name) sql

docker-test-up:
	docker compose --profile test up -d

docker-test-down:
	docker compose --profile test down -v

docker-prod:
	docker compose --profile prod up -d

test-integration:
	go test ./... \
		-tags=integration \
		-v \
		-count=1 \
		-timeout=5m

test-e2e:
	go test ./... \
		-tags=e2e \
		-v \
		-count=1 \
		-timeout=60s

test:
	go test ./... -v -count=1
