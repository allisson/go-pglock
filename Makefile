lint:
	golangci-lint run -v

test:
	go test -covermode=count -coverprofile=count.out -v ./...

test-race:
	go test -race -covermode=atomic -coverprofile=count.out -v ./...

test-coverage:
	go test -covermode=count -coverprofile=count.out -v ./...
	go tool cover -html=count.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

docker-up:
	docker-compose up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3

docker-down:
	docker-compose down -v

test-local: docker-up
	@echo "Running tests with local PostgreSQL..."
	@DATABASE_URL='postgres://test:test@localhost:5432/pglock?sslmode=disable' go test -v ./...
	@$(MAKE) docker-down

mock:
	@rm -rf mocks
	mockery --name Locker

.PHONY: lint test test-race test-coverage docker-up docker-down test-local mock
