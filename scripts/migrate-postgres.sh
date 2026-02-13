#!/bin/bash

# PostgreSQL 数据库迁移脚本
# 使用方法: ./scripts/migrate-postgres.sh [up|down|reset]

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 默认参数
ACTION=${1:-up}
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-postgres}
DB_NAME=${DB_NAME:-aetherflow}

echo "================================"
echo "  PostgreSQL 数据库迁移"
echo "================================"
echo ""
echo "数据库: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
echo "操作: $ACTION"
echo ""

# 检查 PostgreSQL 连接
echo "检查数据库连接..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d postgres -c '\q' 2>/dev/null; then
    echo "✅ 数据库连接成功"
else
    echo "❌ 数据库连接失败！"
    echo ""
    echo "请确保："
    echo "  1. PostgreSQL 已启动"
    echo "  2. 连接参数正确"
    echo "  3. 用户有足够权限"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST=$DB_HOST"
    echo "  DB_PORT=$DB_PORT"
    echo "  DB_USER=$DB_USER"
    echo "  DB_NAME=$DB_NAME"
    exit 1
fi
echo ""

# 创建数据库（如果不存在）
echo "检查数据库..."
if PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -lqt | cut -d \| -f 1 | grep -qw $DB_NAME; then
    echo "✅ 数据库已存在: $DB_NAME"
else
    echo "⚠️  数据库不存在，正在创建..."
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE $DB_NAME;"
    echo "✅ 数据库创建成功"
fi
echo ""

# 执行迁移
case $ACTION in
    up)
        echo "🚀 执行迁移（UP）..."
        echo ""
        
        # 执行初始 Schema
        echo "1️⃣  应用初始 Schema..."
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f deployments/postgres/migrations/001_initial_schema.up.sql
        echo "✅ Schema 创建成功"
        echo ""
        
        # 验证表
        echo "2️⃣  验证表结构..."
        TABLE_COUNT=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE';")
        echo "   创建的表数量: $TABLE_COUNT"
        
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "\dt"
        echo ""
        
        echo "✅ 迁移完成！"
        ;;
        
    down)
        echo "⬇️  回滚迁移（DOWN）..."
        echo ""
        
        echo "⚠️  警告：这将删除所有数据！"
        read -p "确认继续？(yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "❌ 取消操作"
            exit 0
        fi
        
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f deployments/postgres/migrations/001_initial_schema.down.sql
        echo "✅ 回滚完成"
        ;;
        
    reset)
        echo "🔄 重置数据库..."
        echo ""
        
        echo "⚠️  警告：这将删除所有数据并重新创建！"
        read -p "确认继续？(yes/no): " confirm
        if [ "$confirm" != "yes" ]; then
            echo "❌ 取消操作"
            exit 0
        fi
        
        # Down
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f deployments/postgres/migrations/001_initial_schema.down.sql 2>/dev/null || true
        
        # Up
        PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f deployments/postgres/migrations/001_initial_schema.up.sql
        
        echo "✅ 重置完成"
        ;;
        
    *)
        echo "❌ 未知操作: $ACTION"
        echo ""
        echo "用法: $0 [up|down|reset]"
        echo "  up    - 应用迁移"
        echo "  down  - 回滚迁移"
        echo "  reset - 重置数据库"
        exit 1
        ;;
esac

echo ""
echo "================================"
echo "  数据库信息"
echo "================================"
echo ""
echo "连接字符串:"
echo "  postgresql://$DB_USER:****@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable"
echo ""
echo "查看表:"
echo "  psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '\\dt'"
echo ""
echo "查看数据:"
echo "  psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c 'SELECT * FROM documents;'"
echo ""
