#!/bin/bash

# AetherFlow E2E 演示 - 一键启动脚本
# 使用方法: ./start-all.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cd "$E2E_DIR"

echo "================================"
echo "  AetherFlow E2E 演示启动"
echo "================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查 Docker
echo "检查 Docker..."
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker 未安装${NC}"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo -e "${RED}❌ Docker daemon 未运行${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Docker 已就绪${NC}"
echo ""

# 检查 Docker Compose
echo "检查 Docker Compose..."
if ! command -v docker-compose &> /dev/null; then
    echo -e "${YELLOW}⚠️  docker-compose 未安装，尝试使用 docker compose${NC}"
    COMPOSE_CMD="docker compose"
else
    COMPOSE_CMD="docker-compose"
fi
echo -e "${GREEN}✅ Docker Compose 已就绪${NC}"
echo ""

# 停止旧容器
echo "清理旧容器..."
$COMPOSE_CMD down -v 2>/dev/null || true
echo ""

# 构建镜像（如果需要）
echo "检查镜像..."
if [[ "${BUILD_IMAGES:-false}" == "true" ]]; then
    echo "构建服务镜像..."
    cd "$PROJECT_ROOT"
    make build-gateway build-session build-statesync
    cd "$E2E_DIR"
fi
echo ""

# 启动基础设施
echo -e "${YELLOW}🚀 启动基础设施（Redis, PostgreSQL, Jaeger）...${NC}"
$COMPOSE_CMD up -d redis postgres jaeger

# 等待基础设施就绪
echo ""
echo "等待基础设施就绪..."
sleep 5

# 检查 Redis
echo -n "  Redis: "
if docker exec e2e-redis redis-cli ping &> /dev/null; then
    echo -e "${GREEN}✅ Ready${NC}"
else
    echo -e "${RED}❌ Failed${NC}"
    exit 1
fi

# 检查 PostgreSQL
echo -n "  PostgreSQL: "
if docker exec e2e-postgres pg_isready -U postgres &> /dev/null; then
    echo -e "${GREEN}✅ Ready${NC}"
else
    echo -e "${RED}❌ Failed${NC}"
    exit 1
fi

# 检查 Jaeger
echo -n "  Jaeger: "
if curl -s http://localhost:16686/api/services &> /dev/null; then
    echo -e "${GREEN}✅ Ready${NC}"
else
    echo -e "${YELLOW}⚠️  Starting...${NC}"
    sleep 5
fi

echo ""

# 运行数据库迁移
echo -e "${YELLOW}📊 运行数据库迁移...${NC}"
docker exec e2e-postgres psql -U postgres -c "CREATE DATABASE IF NOT EXISTS aetherflow;" 2>/dev/null || true

# 执行 Schema（使用 docker cp + exec）
if [ -f "$PROJECT_ROOT/deployments/postgres/schema.sql" ]; then
    docker cp "$PROJECT_ROOT/deployments/postgres/schema.sql" e2e-postgres:/tmp/schema.sql
    docker exec e2e-postgres psql -U postgres -d aetherflow -f /tmp/schema.sql &> /dev/null || true
    echo -e "${GREEN}✅ Schema 已创建${NC}"
fi
echo ""

# 启动后端服务
echo -e "${YELLOW}🚀 启动后端服务（Gateway, Session, StateSync）...${NC}"
$COMPOSE_CMD up -d gateway session-service statesync-service

# 等待服务启动
echo ""
echo "等待服务就绪（约 15 秒）..."
sleep 10

# 健康检查
echo ""
echo "健康检查:"

check_service() {
    local name=$1
    local url=$2
    local max_attempts=10
    local attempt=0
    
    echo -n "  $name: "
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$url" &> /dev/null; then
            echo -e "${GREEN}✅ Healthy${NC}"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    
    echo -e "${YELLOW}⚠️  Not responding${NC}"
    return 1
}

check_service "Gateway" "http://localhost:8000/health"
check_service "Session Metrics" "http://localhost:9101/metrics"
check_service "StateSync Metrics" "http://localhost:9102/metrics"

echo ""

# 启动 Web 服务器（如果 web 目录存在）
if [ -d "$E2E_DIR/web" ]; then
    echo -e "${YELLOW}🌐 启动 Web 服务器...${NC}"
    
    # 检查是否有 Python
    if command -v python3 &> /dev/null; then
        cd "$E2E_DIR/web"
        nohup python3 -m http.server 8080 > /tmp/e2e-web.log 2>&1 &
        WEB_PID=$!
        echo $WEB_PID > /tmp/e2e-web.pid
        echo -e "${GREEN}✅ Web 服务器已启动 (PID: $WEB_PID)${NC}"
        cd "$E2E_DIR"
    else
        echo -e "${YELLOW}⚠️  Python3 未安装，跳过 Web 服务器${NC}"
    fi
fi

echo ""
echo "================================"
echo -e "${GREEN}  ✅ 所有服务已启动！${NC}"
echo "================================"
echo ""

echo "📊 服务地址:"
echo "  - Gateway:        http://localhost:8000"
echo "  - Web UI:         http://localhost:8080"
echo "  - Jaeger UI:      http://localhost:16686"
echo "  - Prometheus:     http://localhost:9090"
echo "  - Session API:    localhost:9001 (gRPC)"
echo "  - StateSync API:  localhost:9002 (gRPC)"
echo ""

echo "📈 监控指标:"
echo "  - Gateway Metrics:    http://localhost:8100/metrics"
echo "  - Session Metrics:    http://localhost:9101/metrics"
echo "  - StateSync Metrics:  http://localhost:9102/metrics"
echo ""

echo "🎬 运行演示:"
echo "  - 协作演示:  go run demo-collaboration.go"
echo "  - 冲突演示:  go run demo-conflict.go"
echo "  - 性能测试:  go run demo-performance.go"
echo "  - Web UI:    open http://localhost:8080"
echo ""

echo "📝 查看日志:"
echo "  - 所有服务:  docker-compose logs -f"
echo "  - Gateway:   docker-compose logs -f gateway"
echo "  - Session:   docker-compose logs -f session-service"
echo "  - StateSync: docker-compose logs -f statesync-service"
echo ""

echo "🛑 停止服务:"
echo "  ./stop-all.sh"
echo ""

echo "================================"
echo -e "${GREEN}演示准备就绪！${NC}"
echo "================================"
echo ""

# 自动打开浏览器（可选）
if [[ "${OPEN_BROWSER:-false}" == "true" ]]; then
    echo "打开浏览器..."
    if command -v xdg-open &> /dev/null; then
        xdg-open http://localhost:8080 &
    elif command -v open &> /dev/null; then
        open http://localhost:8080 &
    fi
fi

# 保持脚本运行，显示日志（可选）
if [[ "${FOLLOW_LOGS:-false}" == "true" ]]; then
    echo "按 Ctrl+C 退出日志查看"
    echo ""
    $COMPOSE_CMD logs -f
fi
