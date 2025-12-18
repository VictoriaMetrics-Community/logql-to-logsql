UI_DIR=cmd/logql-to-logsql/web/ui

.PHONY: ui-install ui-build build backend-build run test all check lint

ui-install:
	cd $(UI_DIR) && npm install

ui-build: ui-install
	cd $(UI_DIR) && npm run build

backend-build:
	go build -v ./cmd/logql-to-logsql

build: ui-build backend-build

run: ui-build
	go run ./cmd/logql-to-logsql -config ./config.json

test: ui-build
	go test ./...

check:
	bash ./scripts/check-all.sh

lint:
	bash ./scripts/lint-all.sh

all: test check lint build

