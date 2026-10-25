DATABASE_URI ?= postgres://gofermart:gofermart@localhost:5432/gofermart?sslmode=disable

.PHONY: build test cover vet run postgres-up run-loyalty run-accrual compose-up compose-down frontend-install frontend-dev frontend-build

build:
	go build -o bin/loyalty ./cmd/loyalty
	go build -o bin/accrual ./cmd/accrual

test:
	go test ./...

cover:
	go test ./... -coverprofile=cover.out
	go tool cover -func=cover.out | tail -1

format:
	go vet ./...

postgres-up:
	podman compose up -d --wait postgres

run:
	@$(MAKE) postgres-up
	@trap 'trap - INT TERM; kill 0' INT TERM; \
	$(MAKE) run-loyalty & \
	$(MAKE) run-accrual & \
	$(MAKE) frontend-dev & \
	wait

run-loyalty:
	go run ./cmd/loyalty -a localhost:8080 -d '$(DATABASE_URI)' -r http://localhost:8081

run-accrual:
	go run ./cmd/accrual -a localhost:8081 -d '$(DATABASE_URI)'

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down -v

frontend-install:
	pnpm -C frontend install

frontend-dev:
	pnpm -C frontend dev

frontend-build:
	pnpm -C frontend build
