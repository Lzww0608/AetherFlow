#!/bin/bash

# AetherFlow 压力测试脚本
# 用法: ./scripts/stress-test.sh [scenario]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Gateway 地址
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8080}"

# 测试场景
SCENARIO="${1:-basic}"

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}AetherFlow 压力测试${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# 检查压力测试工具是否存在
STRESS_TEST_BIN="./tools/stress-test/stress-test"
if [ ! -f "$STRESS_TEST_BIN" ]; then
    echo -e "${YELLOW}编译压力测试工具...${NC}"
    cd tools/stress-test
    go build -o stress-test main.go
    cd ../..
    echo -e "${GREEN}✓ 编译完成${NC}"
fi

# 检查 Gateway 是否运行
echo -n "检查 Gateway 状态... "
if ! curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}失败${NC}"
    echo "请先启动 Gateway: cd cmd/gateway && go run main.go"
    exit 1
fi
echo -e "${GREEN}成功${NC}"
echo ""

# 运行测试场景
case "$SCENARIO" in
    "basic")
        echo -e "${BLUE}场景 1: 基础负载测试${NC}"
        echo "  - 并发: 10"
        echo "  - 持续时间: 30秒"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 10 \
            -d 30s \
            -method GET
        ;;
        
    "medium")
        echo -e "${BLUE}场景 2: 中等负载测试${NC}"
        echo "  - 并发: 50"
        echo "  - 持续时间: 1分钟"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 50 \
            -d 1m \
            -method GET
        ;;
        
    "heavy")
        echo -e "${BLUE}场景 3: 高负载测试${NC}"
        echo "  - 并发: 100"
        echo "  - 持续时间: 2分钟"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 100 \
            -d 2m \
            -method GET
        ;;
        
    "spike")
        echo -e "${BLUE}场景 4: 峰值测试${NC}"
        echo "  - 并发: 200"
        echo "  - 持续时间: 30秒"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 200 \
            -d 30s \
            -method GET
        ;;
        
    "sustained")
        echo -e "${BLUE}场景 5: 持续负载测试${NC}"
        echo "  - 并发: 50"
        echo "  - 持续时间: 5分钟"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 50 \
            -d 5m \
            -method GET
        ;;
        
    "ratelimit")
        echo -e "${BLUE}场景 6: 限流测试${NC}"
        echo "  - 并发: 20"
        echo "  - RPS: 1000"
        echo "  - 持续时间: 1分钟"
        echo "  - 目标: /health"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/health" \
            -c 20 \
            -rps 1000 \
            -d 1m \
            -method GET
        ;;
        
    "auth")
        echo -e "${BLUE}场景 7: 认证端点测试${NC}"
        echo "  - 并发: 30"
        echo "  - 持续时间: 1分钟"
        echo "  - 目标: /api/v1/auth/login"
        echo ""
        $STRESS_TEST_BIN \
            -target "$GATEWAY_URL/api/v1/auth/login" \
            -c 30 \
            -d 1m \
            -method POST \
            -body '{"username":"testuser","password":"testpass"}'
        ;;
        
    "websocket")
        echo -e "${BLUE}场景 8: WebSocket 连接测试${NC}"
        echo "  - 并发: 100"
        echo "  - 持续时间: 2分钟"
        echo ""
        echo -e "${YELLOW}注意: WebSocket 测试需要专用工具${NC}"
        echo "使用: wscat 或自定义 WebSocket 客户端"
        ;;
        
    "all")
        echo -e "${BLUE}运行所有测试场景${NC}"
        echo ""
        
        for test_scenario in basic medium heavy spike sustained ratelimit auth; do
            echo ""
            echo -e "${YELLOW}================================${NC}"
            bash "$0" "$test_scenario"
            echo ""
            sleep 5
        done
        ;;
        
    *)
        echo -e "${RED}未知场景: $SCENARIO${NC}"
        echo ""
        echo "可用场景:"
        echo "  basic      - 基础负载测试 (10并发, 30秒)"
        echo "  medium     - 中等负载测试 (50并发, 1分钟)"
        echo "  heavy      - 高负载测试 (100并发, 2分钟)"
        echo "  spike      - 峰值测试 (200并发, 30秒)"
        echo "  sustained  - 持续负载测试 (50并发, 5分钟)"
        echo "  ratelimit  - 限流测试 (20并发, 1000 RPS)"
        echo "  auth       - 认证端点测试 (30并发, 1分钟)"
        echo "  websocket  - WebSocket 连接测试"
        echo "  all        - 运行所有测试场景"
        echo ""
        echo "用法: $0 [scenario]"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}测试完成！${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
echo "查看 Prometheus 指标:"
echo "  curl $GATEWAY_URL/metrics"
echo ""
echo "查看 Grafana 仪表盘:"
echo "  http://localhost:3000"
echo ""
