#!/bin/bash

# AetherFlow 启动脚本 - 使用 Redis 启动所有服务
# 使用方法: ./scripts/start-with-redis.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow 启动（Redis模式）"
echo "================================"
echo ""

# 检查 Redis 是否运行
echo "检查 Redis 状态..."
if redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis 已运行"
else
    echo "❌ Redis 未运行，请先启动 Redis："
    echo "   方式1: redis-server"
    echo "   方式2: docker run -d -p 6379:6379 redis:7-alpine"
    echo "   方式3: docker-compose -f deployments/docker-compose.redis.yml up -d redis"
    exit 1
fi
echo ""

# 检查是否已编译
if [ ! -f "bin/session-service" ] || [ ! -f "bin/statesync-service" ] || [ ! -f "bin/gateway" ]; then
    echo "⚠️  检测到未编译的服务，开始编译..."
    make build
    echo ""
fi

# 创建日志目录
mkdir -p logs

# 确保配置文件使用 Redis
echo "检查配置文件..."
if ! grep -q "Type: redis" configs/session.yaml; then
    echo "⚠️  配置文件未设置为 Redis 模式"
    echo "   请编辑 configs/session.yaml，设置 Store.Type: redis"
    exit 1
fi
echo "✅ 配置文件正确"
echo ""

# 启动 Session Service (with Redis)
echo "🚀 启动 Session Service (Redis Store)..."
nohup ./bin/session-service -f configs/session.yaml > logs/session.log 2>&1 &
SESSION_PID=$!
echo "   Session Service PID: $SESSION_PID"
echo ""

# 等待 Session Service 启动
sleep 2

# 验证 Session Service 连接
echo "验证 Session Service..."
if ps -p $SESSION_PID > /dev/null; then
    echo "✅ Session Service 运行中"
    
    # 检查 Redis 连接
    sleep 1
    if tail -10 logs/session.log | grep -q "Using RedisStore"; then
        echo "✅ 已连接到 Redis"
    else
        echo "⚠️  可能未连接到 Redis，请检查日志"
    fi
else
    echo "❌ Session Service 启动失败，查看日志："
    tail -20 logs/session.log
    exit 1
fi
echo ""

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

echo "✅ 所有服务已启动（Redis模式）！"
echo ""
echo "================================"
echo "  服务地址"
echo "================================"
echo "  - Gateway:         http://localhost:8080"
echo "  - Session gRPC:    localhost:9001"
echo "  - StateSync gRPC:  localhost:9002"
echo ""
echo "================================"
echo "  监控地址"
echo "================================"
echo "  - Gateway Metrics:    http://localhost:8081/metrics"
echo "  - Session Metrics:    http://localhost:9101/metrics"
echo "  - StateSync Metrics:  http://localhost:9102/metrics"
echo ""
echo "================================"
echo "  Redis 管理"
echo "================================"
echo "  - Redis CLI:          redis-cli"
echo "  - 查看所有会话:       redis-cli SMEMBERS sessions:all"
echo "  - 查看会话计数:       redis-cli GET sessions:count"
echo "  - 查看特定会话:       redis-cli GET session:SESSION_ID"
echo ""
echo "================================"
echo "  日志文件"
echo "================================"
echo "  - Gateway:    logs/gateway.log"
echo "  - Session:    logs/session.log"
echo "  - StateSync:  logs/statesync.log"
echo ""
echo "使用 ./scripts/stop-all.sh 停止所有服务"
echo "使用 ./scripts/status.sh 查看服务状态"
echo "使用 tail -f logs/session.log 查看实时日志"
