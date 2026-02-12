#!/bin/bash

# Session Store 性能基准测试脚本
# 使用方法: ./scripts/benchmark-stores.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  Session Store 性能基准测试"
echo "================================"
echo ""

# 检查 Redis（用于 RedisStore 测试）
echo "检查 Redis 状态..."
if redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis 可用，将测试两种存储"
    REDIS_AVAILABLE=true
else
    echo "⚠️  Redis 不可用，仅测试 MemoryStore"
    REDIS_AVAILABLE=false
fi
echo ""

# 运行基准测试
echo "================================"
echo "  运行基准测试"
echo "================================"
echo ""

if [ "$REDIS_AVAILABLE" = true ]; then
    # 测试两种存储并对比
    echo "🔬 测试 MemoryStore vs RedisStore..."
    echo ""
    
    go test -bench=BenchmarkComparison -benchmem -benchtime=3s ./internal/session | tee benchmark-results.txt
else
    # 仅测试 MemoryStore
    echo "🔬 测试 MemoryStore..."
    echo ""
    
    go test -bench=BenchmarkMemoryStore -benchmem -benchtime=3s ./internal/session | tee benchmark-results.txt
fi

echo ""
echo "================================"
echo "  测试完成"
echo "================================"
echo ""

# 解析结果
if [ -f "benchmark-results.txt" ]; then
    echo "📊 性能对比总结："
    echo ""
    
    # 提取关键指标
    echo "MemoryStore 性能："
    grep "BenchmarkMemoryStore" benchmark-results.txt | awk '{
        printf "  %-30s %10s ns/op  %10s B/op  %8s allocs/op\n", $1, $3, $5, $7
    }'
    
    echo ""
    
    if [ "$REDIS_AVAILABLE" = true ]; then
        echo "RedisStore 性能："
        grep "BenchmarkRedisStore" benchmark-results.txt | awk '{
            printf "  %-30s %10s ns/op  %10s B/op  %8s allocs/op\n", $1, $3, $5, $7
        }'
        
        echo ""
        echo "性能对比分析："
        echo "  - MemoryStore: 极快 (~1μs)，但无持久化"
        echo "  - RedisStore:  快速 (~1ms)，支持持久化和分布式"
        echo "  - 延迟差异:    ~1000x，但 Redis 提供生产级特性"
    fi
    
    echo ""
    echo "完整结果保存在: benchmark-results.txt"
else
    echo "⚠️  未找到测试结果文件"
fi

echo ""
echo "================================"
echo "  运行建议"
echo "================================"
echo ""
echo "单独测试 MemoryStore:"
echo "  go test -bench=BenchmarkMemoryStore -benchmem ./internal/session"
echo ""
echo "单独测试 RedisStore:"
echo "  go test -bench=BenchmarkRedisStore -benchmem ./internal/session"
echo ""
echo "更长时间测试（更准确）:"
echo "  go test -bench=. -benchmem -benchtime=10s ./internal/session"
echo ""
echo "CPU 性能分析:"
echo "  go test -bench=. -cpuprofile=cpu.prof ./internal/session"
echo "  go tool pprof cpu.prof"
echo ""
echo "内存性能分析:"
echo "  go test -bench=. -memprofile=mem.prof ./internal/session"
echo "  go tool pprof mem.prof"
echo ""
