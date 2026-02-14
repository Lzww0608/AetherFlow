#!/bin/bash

# AetherFlow E2E 演示 - 停止脚本

set -e

E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$E2E_DIR"

echo "================================"
echo "  停止 AetherFlow E2E 演示"
echo "================================"
echo ""

# 检查 Docker Compose 命令
if ! command -v docker-compose &> /dev/null; then
    COMPOSE_CMD="docker compose"
else
    COMPOSE_CMD="docker-compose"
fi

# 停止 Docker Compose 服务
echo "停止 Docker Compose 服务..."
$COMPOSE_CMD down

# 停止 Web 服务器
if [ -f /tmp/e2e-web.pid ]; then
    WEB_PID=$(cat /tmp/e2e-web.pid)
    if ps -p $WEB_PID > /dev/null 2>&1; then
        echo "停止 Web 服务器 (PID: $WEB_PID)..."
        kill $WEB_PID
        rm /tmp/e2e-web.pid
    fi
fi

echo ""
echo "✅ 所有服务已停止"
echo ""
