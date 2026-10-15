Write-Host "Starting IntelliFinance Homelab Infrastructure (PostgreSQL 16 & Redis 7)..." -ForegroundColor Cyan
docker compose up -d

Write-Host "Checking service health status..." -ForegroundColor Cyan
docker compose ps
