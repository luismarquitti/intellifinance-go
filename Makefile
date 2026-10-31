SHELL := /bin/bash
.DEFAULT_GOAL := help

.PHONY: help build build-web build-api mvp-up mvp-down mvp-status mvp-logs mvp-seed pg-up pg-down test lint smoke smoke-browser

help:
	@printf '%s\n' 'build: compile dashboard and API' 'mvp-up/down/status/logs: user-level API service (no sudo)' 'pg-up/down: operate the prepared isolated PostgreSQL cluster' 'mvp-seed: idempotent development seed' 'test: real database tests with race detector (separate test.env)' 'lint: golangci-lint v2' 'smoke: real HTTP import/CRUD/deduplication checks' 'smoke-browser: optional Playwright dashboard checks'

build: build-api
build-api: build-web
	@bash scripts/local-dev.sh build-api
build-web:
	@bash scripts/local-dev.sh build-web
mvp-up: build-api
	@bash scripts/local-dev.sh up
mvp-down:
	@bash scripts/local-dev.sh down
mvp-status:
	@bash scripts/local-dev.sh status
mvp-logs:
	@bash scripts/local-dev.sh logs
mvp-seed:
	@bash scripts/local-dev.sh seed
pg-up:
	@bash scripts/local-dev.sh pg-up
pg-down:
	@bash scripts/local-dev.sh pg-down
test: test-scripts
	@bash scripts/local-dev.sh test
.PHONY: test-scripts
test-scripts:
	@PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s test -p '*_test.py' -v
lint:
	@bash scripts/local-dev.sh lint
smoke:
	@python3 scripts/smoke-local.py
smoke-browser:
	@node scripts/smoke-browser.cjs
