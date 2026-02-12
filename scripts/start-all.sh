#!/bin/bash

# AetherFlow 启动脚本 - 启动所有服务
# 使用方法: ./scripts/start-all.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow 启动所有服务"
echo "================================"
echo ""

# 检查是否已编译
if [ ! -f "bin/gateway" ] || [ ! -f "bin/session-service" ] || [ ! -f "bin/statesync-service" ]; then
    echo "⚠️  检测到未编译的服务，开始编译..."
    make build
    echo ""
fi

# 启动 Session Service
echo "🚀 启动 Session Service..."
nohup ./bin/session-service -f configs/session.yaml > logs/session.log 2>&1 &
SESSION_PID=$!
echo "   Session Service PID: $SESSION_PID"
echo ""

# 等待 Session Service 启动
sleep 2

# 启动 StateSync Service
echo "🚀 启动 StateSync Service..."
nohup ./bin/statesync-service -f configs/statesync.yaml > logs/statesync.log 2>&1 &
STATESYNC_PID=$!
echo "   StateSync Service PID: $STATESYNC_PID"
echo ""

# 等待 StateSync Service 启动
sleep 2

# 启动 Gateway
echo "🚀 启动 Gateway..."
nohup ./bin/gateway -f configs/gateway.yaml > logs/gateway.log 2>&1 &
GATEWAY_PID=$!
echo "   Gateway PID: $GATEWAY_PID"
echo ""

# 保存 PID
mkdir -p .runtime
echo "$SESSION_PID" > .runtime/session.pid
echo "$STATESYNC_PID" > .runtime/statesync.pid
echo "$GATEWAY_PID" > .runtime/gateway.pid

echo "✅ 所有服务已启动！"
echo ""
echo "服务地址："
echo "  - Gateway:         http://localhost:8080"
echo "  - Session gRPC:    localhost:9001"
echo "  - StateSync gRPC:  localhost:9002"
echo ""
echo "监控地址："
echo "  - Gateway Metrics:    http://localhost:8081/metrics"
echo "  - Session Metrics:    http://localhost:9101/metrics"
echo "  - StateSync Metrics:  http://localhost:9102/metrics"
echo ""
echo "日志文件："
echo "  - Gateway:    logs/gateway.log"
echo "  - Session:    logs/session.log"
echo "  - StateSync:  logs/statesync.log"
echo ""
echo "使用 ./scripts/stop-all.sh 停止所有服务"
echo "使用 ./scripts/status.sh 查看服务状态"
