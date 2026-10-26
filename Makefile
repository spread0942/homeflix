# Homeflix — Compose helpers
#
# Docker (port 80):     make up / make down
# Podman (port 8080):   make podman-up / make podman-down
# Prod-style (no Vite): make up-prod / make podman-up-prod

COMPOSE       ?= docker compose
PODMAN_COMPOSE ?= podman compose

COMPOSE_DEV_FILES := -f docker-compose.yml -f docker-compose.override.yml
COMPOSE_PROD_FILES := -f docker-compose.yml
PODMAN_DEV_FILES := $(COMPOSE_DEV_FILES) -f docker-compose.podman.yml
PODMAN_PROD_FILES := $(COMPOSE_PROD_FILES) -f docker-compose.podman.yml

.PHONY: up down build logs ps restart up-prod down-prod \
	podman-up podman-down podman-build podman-logs podman-ps podman-restart \
	podman-up-prod podman-down-prod help

help:
	@echo "Docker:  make up | down | build | logs | ps | up-prod | down-prod"
	@echo "Podman:  make podman-up | podman-down | podman-build | podman-logs | podman-ps | podman-up-prod"
	@echo "Podman serves http://localhost:8080 (rootless)."

# --- Docker (hot reload) ---
up:
	$(COMPOSE) $(COMPOSE_DEV_FILES) up -d --build

down:
	$(COMPOSE) $(COMPOSE_DEV_FILES) down

build:
	$(COMPOSE) $(COMPOSE_DEV_FILES) build

logs:
	$(COMPOSE) $(COMPOSE_DEV_FILES) logs -f

ps:
	$(COMPOSE) $(COMPOSE_DEV_FILES) ps

restart:
	$(COMPOSE) $(COMPOSE_DEV_FILES) restart

# --- Docker (production-style nginx frontend) ---
up-prod:
	$(COMPOSE) $(COMPOSE_PROD_FILES) up -d --build

down-prod:
	$(COMPOSE) $(COMPOSE_PROD_FILES) down

# --- Podman (hot reload, port 8080) ---
podman-up:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) up -d --build

podman-down:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) down

podman-build:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) build

podman-logs:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) logs -f

podman-ps:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) ps

podman-restart:
	$(PODMAN_COMPOSE) $(PODMAN_DEV_FILES) restart

# --- Podman (production-style) ---
podman-up-prod:
	$(PODMAN_COMPOSE) $(PODMAN_PROD_FILES) up -d --build

podman-down-prod:
	$(PODMAN_COMPOSE) $(PODMAN_PROD_FILES) down
