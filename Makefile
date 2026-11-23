.PHONY: db-up db-down run build test tidy up down logs tunnel

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

run:
	go run ./cmd/api

build:
	go build -o bin/temuin-api ./cmd/api

test:
	go test ./...

tidy:
	go mod tidy

up:            ## build & jalankan API + Postgres di Docker
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f api

tunnel:        ## expose API ke internet via ngrok (set BASE_URL=URL ngrok di .env lalu `make up`)
	ngrok http 8080
