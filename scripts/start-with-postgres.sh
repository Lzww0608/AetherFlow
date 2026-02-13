#!/bin/bash

# AetherFlow 启动脚本 - 使用 PostgreSQL 启动 StateSync Service
# 使用方法: ./scripts/start-with-postgres.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow 启动（PostgreSQL模式）"
echo "================================"
echo ""

# PostgreSQL 配置
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-aetherflow}

# 检查 PostgreSQL 是否运行
echo "检查 PostgreSQL 状态..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d postgres -c '\q' 2>/dev/null; then
    echo "✅ PostgreSQL 已运行"
else
    echo "❌ PostgreSQL 未运行，请先启动 PostgreSQL："
    echo "   方式1: pg_ctl start -D /usr/local/var/postgres"
    echo "   方式2: docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:15-alpine"
    echo "   方式3: docker-compose -f deployments/docker-compose.postgres.yml up -d postgres"
    exit 1
fi
echo ""

# 检查数据库是否存在
echo "检查数据库..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -lqt | cut -d \| -f 1 | grep -qw $DB_NAME; then
    echo "✅ 数据库已存在: $DB_NAME"
else
    echo "⚠️  数据库不存在，正在创建..."
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE $DB_NAME;"
    echo "✅ 数据库创建成功"
    
    echo ""
    echo "运行数据库迁移..."
    ./scripts/migrate-postgres.sh up
fi
echo ""

# 检查是否已编译
if [ ! -f "bin/statesync-service" ]; then
    echo "⚠️  检测到未编译的服务，开始编译..."
    make build-statesync
    echo ""
fi

# 创建日志目录
mkdir -p logs

# 确保配置文件使用 PostgreSQL
echo "检查配置文件..."
if ! grep -q "Type: postgres" configs/statesync.yaml; then
    echo "⚠️  配置文件未设置为 PostgreSQL 模式"
    echo "   请编辑 configs/statesync.yaml，设置 Store.Type: postgres"
    echo ""
    echo "示例配置:"
    echo "Store:"
    echo "  Type: postgres"
    echo "  Postgres:"
    echo "    Host: localhost"
    echo "    Port: 5432"
    echo "    User: postgres"
    echo "    Password: postgres"
    echo "    DBName: aetherflow"
    exit 1
fi
echo "✅ 配置文件正确"
echo ""

# 启动 Session Service（如果需要）
if [ ! -f ".runtime/session.pid" ] || ! ps -p $(cat .runtime/session.pid 2>/dev/null) > /dev/null 2>&1; then
    if [ -f "bin/session-service" ]; then
        echo "🚀 启动 Session Service..."
        nohup ./bin/session-service -f configs/session.yaml > logs/session.log 2>&1 &
        SESSION_PID=$!
        mkdir -p .runtime
        echo "$SESSION_PID" > .runtime/session.pid
        echo "   Session Service PID: $SESSION_PID"
        sleep 2
        echo ""
    fi
fi

# 启动 StateSync Service (with PostgreSQL)
echo "🚀 启动 StateSync Service (PostgreSQL Store)..."
nohup ./bin/statesync-service -f configs/statesync.yaml > logs/statesync.log 2>&1 &
STATESYNC_PID=$!
echo "   StateSync Service PID: $STATESYNC_PID"
echo ""

# 等待 StateSync Service 启动
sleep 3

# 验证 StateSync Service 连接
echo "验证 StateSync Service..."
if ps -p $STATESYNC_PID > /dev/null; then
    echo "✅ StateSync Service 运行中"
    
    # 检查 PostgreSQL 连接
    sleep 1
    if tail -10 logs/statesync.log | grep -q "Using PostgresStore"; then
        echo "✅ 已连接到 PostgreSQL"
    else
        echo "⚠️  可能未连接到 PostgreSQL，请检查日志"
    fi
else
    echo "❌ StateSync Service 启动失败，查看日志："
    tail -20 logs/statesync.log
    exit 1
fi
echo ""

# 启动 Gateway（如果需要）
if [ ! -f ".runtime/gateway.pid" ] || ! ps -p $(cat .runtime/gateway.pid 2>/dev/null) > /dev/null 2>&1; then
    if [ -f "bin/gateway" ]; then
        echo "🚀 启动 Gateway..."
        nohup ./bin/gateway -f configs/gateway.yaml > logs/gateway.log 2>&1 &
        GATEWAY_PID=$!
        mkdir -p .runtime
        echo "$GATEWAY_PID" > .runtime/gateway.pid
        echo "   Gateway PID: $GATEWAY_PID"
        sleep 2
        echo ""
    fi
fi

# 保存 PID
mkdir -p .runtime
echo "$STATESYNC_PID" > .runtime/statesync.pid

echo "✅ 服务已启动（PostgreSQL模式）！"
echo ""
echo "================================"
echo "  服务地址"
echo "================================"
echo "  - StateSync gRPC:  localhost:9002"
echo "  - StateSync Metrics: http://localhost:9102/metrics"
echo ""
echo "================================"
echo "  PostgreSQL 管理"
echo "================================"
echo "  - psql 连接:       psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME"
echo "  - 查看文档:        psql ... -c 'SELECT id, name, type FROM documents;'"
echo "  - 查看操作:        psql ... -c 'SELECT id, type, version FROM operations;'"
echo "  - 查看统计:        psql ... -c 'SELECT COUNT(*) FROM documents;'"
echo ""
echo "================================"
echo "  日志文件"
echo "================================"
echo "  - StateSync:  logs/statesync.log"
echo ""
echo "使用 ./scripts/stop-all.sh 停止所有服务"
echo "使用 tail -f logs/statesync.log 查看实时日志"
echo ""
