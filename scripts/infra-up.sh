#!/usr/bin/env bash
set -euo pipefail

echo "Starting IntelliFinance Homelab Infrastructure (PostgreSQL 16 & Redis 7)..."
docker compose up -d

echo "Waiting for services to become healthy..."
docker compose ps
