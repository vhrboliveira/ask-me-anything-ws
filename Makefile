generate:
	go generate ./...

test:
	go test ./...

test-verbose:
	go test -v ./...
	
docker-up:
	docker compose --env-file .env -f deploy/compose.yaml up -d --build

docker-test:
	docker compose -f deploy/compose-test.yaml up -d

docker-down:
	docker compose --env-file .env -f deploy/compose.yaml down

docker-test-down:
	docker compose -f deploy/compose-test.yaml down

docker-logs:
	docker compose -f deploy/compose.yaml logs -f ama-go
