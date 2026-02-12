#!/bin/bash

# AetherFlow 停止脚本 - 停止所有服务
# 使用方法: ./scripts/stop-all.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow 停止所有服务"
echo "================================"
echo ""

# 检查 PID 文件
if [ ! -d ".runtime" ]; then
    echo "⚠️  没有找到运行时信息，服务可能未启动"
    exit 0
fi

# 停止 Gateway
if [ -f ".runtime/gateway.pid" ]; then
    GATEWAY_PID=$(cat .runtime/gateway.pid)
    echo "🛑 停止 Gateway (PID: $GATEWAY_PID)..."
    kill $GATEWAY_PID 2>/dev/null || echo "   Gateway 已经停止"
    rm -f .runtime/gateway.pid
fi

# 停止 StateSync Service
if [ -f ".runtime/statesync.pid" ]; then
    STATESYNC_PID=$(cat .runtime/statesync.pid)
    echo "🛑 停止 StateSync Service (PID: $STATESYNC_PID)..."
    kill $STATESYNC_PID 2>/dev/null || echo "   StateSync Service 已经停止"
    rm -f .runtime/statesync.pid
fi

# 停止 Session Service
if [ -f ".runtime/session.pid" ]; then
    SESSION_PID=$(cat .runtime/session.pid)
    echo "🛑 停止 Session Service (PID: $SESSION_PID)..."
    kill $SESSION_PID 2>/dev/null || echo "   Session Service 已经停止"
    rm -f .runtime/session.pid
fi

# 清理运行时目录
rmdir .runtime 2>/dev/null || true

echo ""
echo "✅ 所有服务已停止！"
