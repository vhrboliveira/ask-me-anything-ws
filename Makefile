generate:
	@go generate ./...

tests:
	@go test ./...

test-verbose:
	@go test -v ./...

run:
	@docker network create ama-shared-network || true
	@docker compose --env-file .prod.env -f deploy/compose.yaml up -d --build

stop:
	@docker compose --env-file .prod.env -f deploy/compose.yaml down
	@docker network rm ama-shared-network || true
	
docker-up:
	@docker network create ama-shared-network || true
	@docker compose --env-file .env -f deploy/compose-dev.yaml up -d --build

docker-test:
	@docker compose -f deploy/compose-test.yaml up -d

docker-down:
	@docker compose --env-file .env -f deploy/compose-dev.yaml down
	@docker network rm ama-shared-network || true

docker-test-down:
	@docker compose -f deploy/compose-test.yaml down

docker-logs:
	@docker compose -f deploy/compose.yaml logs -f ama-go
