# Start DraftRight backend services (postgres, redis, backend) via Docker Compose
# Usage: powershell -ExecutionPolicy Bypass -File start-server.ps1

$ErrorActionPreference = "Stop"
$composeDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

Push-Location $composeDir

function Test-ContainerRunning($service) {
    $state = docker compose ps --format '{{.State}}' $service 2>$null
    return $state -eq "running"
}

function Wait-Healthy($service, $maxWait = 30) {
    for ($i = 0; $i -lt $maxWait; $i++) {
        $health = docker compose ps --format '{{.Health}}' $service 2>$null
        if ($health -eq "healthy" -or $health -eq "") { return $true }
        Start-Sleep -Seconds 1
    }
    return $false
}

Write-Host "=== DraftRight Server Check ===" -ForegroundColor Cyan
Write-Host ""

# 1. PostgreSQL
if (Test-ContainerRunning "postgres") {
    Write-Host "[OK] PostgreSQL is running" -ForegroundColor Green
} else {
    Write-Host "[STARTING] PostgreSQL..." -ForegroundColor Yellow
    docker compose up -d postgres
    Write-Host -NoNewline "  Waiting for healthy..."
    if (Wait-Healthy "postgres" 30) {
        Write-Host " ready" -ForegroundColor Green
    } else {
        Write-Host " timed out" -ForegroundColor Red
        Pop-Location; exit 1
    }
}

# 2. Redis
if (Test-ContainerRunning "redis") {
    Write-Host "[OK] Redis is running" -ForegroundColor Green
} else {
    Write-Host "[STARTING] Redis..." -ForegroundColor Yellow
    docker compose up -d redis
    Start-Sleep -Seconds 1
    Write-Host "[OK] Redis started" -ForegroundColor Green
}

# 3. Backend
if (Test-ContainerRunning "backend") {
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:3000/health" -TimeoutSec 5 -ErrorAction Stop
        Write-Host "[OK] Backend is running and healthy" -ForegroundColor Green
    } catch {
        Write-Host "[RESTARTING] Backend is running but not responding..." -ForegroundColor Yellow
        docker compose restart backend
        Start-Sleep -Seconds 3
        try {
            Invoke-RestMethod -Uri "http://localhost:3000/health" -TimeoutSec 5 -ErrorAction Stop | Out-Null
            Write-Host "[OK] Backend recovered" -ForegroundColor Green
        } catch {
            Write-Host "[FAIL] Backend not responding after restart" -ForegroundColor Red
            Write-Host "  Check logs: docker compose logs backend --tail 20"
            Pop-Location; exit 1
        }
    }
} else {
    Write-Host "[STARTING] Backend..." -ForegroundColor Yellow
    docker compose up -d backend
    Write-Host -NoNewline "  Waiting for API..."
    $ready = $false
    for ($i = 0; $i -lt 15; $i++) {
        try {
            Invoke-RestMethod -Uri "http://localhost:3000/health" -TimeoutSec 3 -ErrorAction Stop | Out-Null
            $ready = $true
            break
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    if ($ready) {
        Write-Host " ready" -ForegroundColor Green
    } else {
        Write-Host " timed out" -ForegroundColor Red
        Write-Host "  Check logs: docker compose logs backend --tail 20"
        Pop-Location; exit 1
    }
}

Write-Host ""
Write-Host "All services are up." -ForegroundColor Green
Pop-Location
