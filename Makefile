SHELL := /bin/sh

API_DIR := backend
WEB_DIR := frontend

.PHONY: help install dev api web build test test-api test-web lint fmt tidy clean

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

install: ## Install root, frontend, and Go dependencies
	npm install
	npm --prefix $(WEB_DIR) install
	cd $(API_DIR) && go mod download

dev: ## Run API and web dev servers together
	npm run dev

api: ## Run only the Go API server
	npm run dev:api

web: ## Run only the Vite dev server
	npm run dev:web

build: ## Build API binary and frontend bundle
	npm run build

test: ## Run all backend and frontend tests
	npm test

test-api: ## Run backend tests
	npm run test:api

test-web: ## Run frontend tests
	npm run test:web

lint: ## Vet Go code and lint the frontend
	npm run lint

fmt: ## Format Go and frontend sources
	npm run fmt

tidy: ## Tidy Go module dependencies
	npm run tidy

clean: ## Remove build output
	rm -rf $(API_DIR)/bin $(WEB_DIR)/dist $(WEB_DIR)/coverage
