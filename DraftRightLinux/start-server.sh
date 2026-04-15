#!/usr/bin/env bash
# Start DraftRight backend services (postgres, redis, backend)
# Usage: ./start-server.sh
# Expects docker-compose.yml one level up from this script's directory.

set -euo pipefail

# Navigate to the project root where docker-compose.yml lives
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COMPOSE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$COMPOSE_DIR"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

check_container() {
    local name="$1"
    local status
    status=$(docker compose ps --format '{{.State}}' "$name" 2>/dev/null)
    [[ "$status" == "running" ]]
}

wait_healthy() {
    local name="$1"
    local max_wait="${2:-30}"
    local elapsed=0
    while (( elapsed < max_wait )); do
        local health
        health=$(docker compose ps --format '{{.Health}}' "$name" 2>/dev/null)
        if [[ "$health" == "healthy" || "$health" == "" ]]; then
            return 0
        fi
        sleep 1
        (( elapsed++ ))
    done
    return 1
}

echo "=== DraftRight Server Check ==="
echo ""

# 1. PostgreSQL
if check_container postgres; then
    echo -e "${GREEN}[OK]${NC} PostgreSQL is running"
else
    echo -e "${YELLOW}[STARTING]${NC} PostgreSQL..."
    docker compose up -d postgres
    echo -n "  Waiting for healthy..."
    if wait_healthy postgres 30; then
        echo -e " ${GREEN}ready${NC}"
    else
        echo -e " ${RED}timed out${NC}"
        exit 1
    fi
fi

# 2. Redis
if check_container redis; then
    echo -e "${GREEN}[OK]${NC} Redis is running"
else
    echo -e "${YELLOW}[STARTING]${NC} Redis..."
    docker compose up -d redis
    sleep 1
    echo -e "${GREEN}[OK]${NC} Redis started"
fi

# 3. Backend
if check_container backend; then
    if curl -sf http://localhost:3000/health > /dev/null 2>&1; then
        echo -e "${GREEN}[OK]${NC} Backend is running and healthy"
    else
        echo -e "${YELLOW}[RESTARTING]${NC} Backend is running but not responding..."
        docker compose restart backend
        sleep 3
        if curl -sf http://localhost:3000/health > /dev/null 2>&1; then
            echo -e "${GREEN}[OK]${NC} Backend recovered"
        else
            echo -e "${RED}[FAIL]${NC} Backend not responding after restart"
            echo "  Check logs: docker compose logs backend --tail 20"
            exit 1
        fi
    fi
else
    echo -e "${YELLOW}[STARTING]${NC} Backend..."
    docker compose up -d backend
    echo -n "  Waiting for API..."
    for i in $(seq 1 15); do
        if curl -sf http://localhost:3000/health > /dev/null 2>&1; then
            echo -e " ${GREEN}ready${NC}"
            break
        fi
        if (( i == 15 )); then
            echo -e " ${RED}timed out${NC}"
            echo "  Check logs: docker compose logs backend --tail 20"
            exit 1
        fi
        sleep 1
    done
fi

echo ""
echo -e "${GREEN}All services are up.${NC}"
