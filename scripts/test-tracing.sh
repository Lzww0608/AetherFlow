#!/bin/bash

# 链路追踪功能测试脚本
# 用法: ./scripts/test-tracing.sh

set -e

echo "================================"
echo "AetherFlow 链路追踪测试"
echo "================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Gateway 地址
GATEWAY_URL="http://localhost:8080"

# 检查 Gateway 是否运行
echo -n "检查 Gateway 状态... "
if ! curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}失败${NC}"
    echo "请先启动 Gateway: cd cmd/gateway && go run main.go"
    exit 1
fi
echo -e "${GREEN}成功${NC}"

# 检查 Jaeger 是否运行
echo -n "检查 Jaeger 状态... "
if ! curl -s "http://localhost:14268/api/traces" > /dev/null 2>&1; then
    echo -e "${YELLOW}警告${NC}"
    echo "Jaeger 未运行。启动命令:"
    echo "docker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest"
else
    echo -e "${GREEN}成功${NC}"
fi

echo ""
echo "步骤 1: 发送健康检查请求"
echo "------------------------"
RESPONSE=$(curl -s -i "$GATEWAY_URL/health")
echo "$RESPONSE" | head -n 15

# 提取 Trace ID
TRACE_ID=$(echo "$RESPONSE" | grep -i "X-Trace-ID:" | cut -d: -f2 | tr -d ' \r')
SPAN_ID=$(echo "$RESPONSE" | grep -i "X-Span-ID:" | cut -d: -f2 | tr -d ' \r')

if [ -n "$TRACE_ID" ]; then
    echo -e "${GREEN}✓ 检测到 Trace ID: $TRACE_ID${NC}"
    echo -e "${GREEN}✓ 检测到 Span ID: $SPAN_ID${NC}"
else
    echo -e "${YELLOW}⚠ 未检测到追踪信息（追踪可能未启用）${NC}"
fi

echo ""
echo "步骤 2: 发送认证请求"
echo "--------------------"
AUTH_RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"testuser","password":"testpass123"}')

echo "$AUTH_RESPONSE" | jq -C '.' 2>/dev/null || echo "$AUTH_RESPONSE"

# 提取 token (如果响应中有)
TOKEN=$(echo "$AUTH_RESPONSE" | jq -r '.data.access_token' 2>/dev/null)

if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
    echo -e "${GREEN}✓ 获取到 Token${NC}"
    
    echo ""
    echo "步骤 3: 发送需要认证的请求"
    echo "----------------------------"
    SESSION_RESPONSE=$(curl -s -i -X POST "$GATEWAY_URL/api/v1/session" \
        -H "Authorization: Bearer $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{
            "user_id": "test-user-123",
            "client_ip": "127.0.0.1",
            "client_port": 12345
        }')
    
    echo "$SESSION_RESPONSE" | head -n 20
    
    SESSION_TRACE_ID=$(echo "$SESSION_RESPONSE" | grep -i "X-Trace-ID:" | cut -d: -f2 | tr -d ' \r')
    if [ -n "$SESSION_TRACE_ID" ]; then
        echo -e "${GREEN}✓ Session 请求 Trace ID: $SESSION_TRACE_ID${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 未能获取认证 token（Session Service 可能未运行）${NC}"
fi

echo ""
echo "步骤 4: 发送多个请求生成追踪数据"
echo "--------------------------------"
for i in {1..5}; do
    curl -s "$GATEWAY_URL/health" > /dev/null
    echo -n "."
done
echo -e " ${GREEN}完成${NC}"

echo ""
echo "================================"
echo "测试完成！"
echo "================================"
echo ""

if [ -n "$TRACE_ID" ]; then
    echo "查看追踪数据:"
    echo "  Jaeger UI: http://localhost:16686"
    echo "  搜索 Service: aetherflow-gateway"
    echo "  Trace ID: $TRACE_ID"
    echo ""
    echo "  直接链接:"
    echo "  http://localhost:16686/trace/$TRACE_ID"
else
    echo "如果追踪功能未启用，请检查 configs/gateway.yaml:"
    echo ""
    echo "  Tracing:"
    echo "    Enable: true"
    echo "    Exporter: jaeger"
    echo "    SampleRate: 1.0"
fi

echo ""
