#!/bin/bash

# AetherFlow 状态检查脚本
# 使用方法: ./scripts/status.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow 服务状态"
echo "================================"
echo ""

# 检查服务函数
check_service() {
    local name=$1
    local pid_file=$2
    local port=$3
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p $pid > /dev/null 2>&1; then
            echo "✅ $name: 运行中 (PID: $pid)"
            
            # 检查端口
            if command -v lsof >/dev/null 2>&1; then
                if lsof -i :$port > /dev/null 2>&1; then
                    echo "   端口 $port: 监听中"
                else
                    echo "   ⚠️  端口 $port: 未监听"
                fi
            fi
        else
            echo "❌ $name: 未运行 (PID 文件存在但进程不存在)"
        fi
    else
        echo "❌ $name: 未运行"
    fi
}

# 检查各服务
check_service "Gateway" ".runtime/gateway.pid" "8080"
echo ""
check_service "Session Service" ".runtime/session.pid" "9001"
echo ""
check_service "StateSync Service" ".runtime/statesync.pid" "9002"
echo ""

# 测试服务连通性
echo "================================"
echo "  服务连通性测试"
echo "================================"
echo ""

# 测试 Gateway
if curl -s -f http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ Gateway HTTP: 可访问"
else
    echo "❌ Gateway HTTP: 不可访问"
fi

# 测试 Session Service (使用 grpcurl 如果可用)
if command -v grpcurl >/dev/null 2>&1; then
    if grpcurl -plaintext localhost:9001 grpc.health.v1.Health/Check > /dev/null 2>&1; then
        echo "✅ Session gRPC: 可访问"
    else
        echo "❌ Session gRPC: 不可访问"
    fi
else
    echo "⚠️  Session gRPC: 无法测试 (需要安装 grpcurl)"
fi

# 测试 StateSync Service
if command -v grpcurl >/dev/null 2>&1; then
    if grpcurl -plaintext localhost:9002 grpc.health.v1.Health/Check > /dev/null 2>&1; then
        echo "✅ StateSync gRPC: 可访问"
    else
        echo "❌ StateSync gRPC: 不可访问"
    fi
else
    echo "⚠️  StateSync gRPC: 无法测试 (需要安装 grpcurl)"
fi

echo ""
echo "================================"
echo "  指标端点"
echo "================================"
echo ""

# 测试 Metrics
if curl -s http://localhost:8081/metrics | grep -q "promhttp_metric_handler_requests_total"; then
    echo "✅ Gateway Metrics: http://localhost:8081/metrics"
else
    echo "❌ Gateway Metrics: 不可访问"
fi

if curl -s http://localhost:9101/metrics | grep -q "promhttp_metric_handler_requests_total"; then
    echo "✅ Session Metrics: http://localhost:9101/metrics"
else
    echo "❌ Session Metrics: 不可访问"
fi

if curl -s http://localhost:9102/metrics | grep -q "promhttp_metric_handler_requests_total"; then
    echo "✅ StateSync Metrics: http://localhost:9102/metrics"
else
    echo "❌ StateSync Metrics: 不可访问"
fi
