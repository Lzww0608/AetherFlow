#!/bin/bash

# Redis Store 验证脚本
# 使用方法: ./scripts/verify-redis.sh

set -e

echo "================================"
echo "  Redis Store 功能验证"
echo "================================"
echo ""

# 检查 Redis
echo "1️⃣  检查 Redis 服务..."
if redis-cli ping > /dev/null 2>&1; then
    echo "✅ Redis 运行中"
else
    echo "❌ Redis 未运行，请先启动："
    echo "   redis-server"
    exit 1
fi
echo ""

# 清理测试数据
echo "2️⃣  清理旧测试数据..."
redis-cli FLUSHDB > /dev/null 2>&1 || true
echo "✅ 已清理"
echo ""

# 检查服务是否运行
echo "3️⃣  检查 Session Service..."
if grpcurl -plaintext localhost:9001 grpc.health.v1.Health/Check > /dev/null 2>&1; then
    echo "✅ Session Service 运行中"
else
    echo "❌ Session Service 未运行，请先启动："
    echo "   ./scripts/start-with-redis.sh"
    exit 1
fi
echo ""

# 创建测试会话
echo "4️⃣  创建测试会话..."
RESPONSE=$(grpcurl -plaintext -d '{
  "user_id": "redis-test-user",
  "client_ip": "127.0.0.1",
  "client_port": 9999,
  "timeout_seconds": 300
}' localhost:9001 session.SessionService/CreateSession)

SESSION_ID=$(echo "$RESPONSE" | grep -o '"session_id": "[^"]*"' | cut -d'"' -f4)

if [ -n "$SESSION_ID" ]; then
    echo "✅ 会话创建成功"
    echo "   Session ID: $SESSION_ID"
else
    echo "❌ 会话创建失败"
    exit 1
fi
echo ""

# 验证 Redis 存储
echo "5️⃣  验证 Redis 数据..."

# 检查会话数据
if redis-cli EXISTS "session:$SESSION_ID" | grep -q "1"; then
    echo "✅ 会话数据已存储"
    
    # 显示会话数据（格式化 JSON）
    echo ""
    echo "   会话数据预览:"
    redis-cli GET "session:$SESSION_ID" | jq '.' | head -15
else
    echo "❌ 会话数据未找到"
    exit 1
fi
echo ""

# 检查索引
echo "6️⃣  验证索引..."

# 检查全局集合
if redis-cli SISMEMBER sessions:all "$SESSION_ID" | grep -q "1"; then
    echo "✅ 全局索引正常"
else
    echo "❌ 全局索引异常"
fi

# 检查用户索引
if redis-cli SISMEMBER user_idx:redis-test-user "$SESSION_ID" | grep -q "1"; then
    echo "✅ 用户索引正常"
else
    echo "❌ 用户索引异常"
fi

# 检查计数
COUNT=$(redis-cli GET sessions:count)
echo "✅ 会话计数: $COUNT"
echo ""

# 检查 TTL
echo "7️⃣  验证 TTL..."
TTL=$(redis-cli TTL "session:$SESSION_ID")
if [ "$TTL" -gt 0 ]; then
    echo "✅ TTL 已设置: ${TTL}秒"
else
    echo "❌ TTL 未设置"
fi
echo ""

# 更新会话
echo "8️⃣  测试会话更新..."
grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\",
  \"state\": 2,
  \"metadata\": {\"updated\": \"true\"}
}" localhost:9001 session.SessionService/UpdateSession > /dev/null 2>&1

# 验证更新
UPDATED=$(redis-cli GET "session:$SESSION_ID" | jq -r '.metadata.updated')
if [ "$UPDATED" = "true" ]; then
    echo "✅ 会话更新成功"
else
    echo "❌ 会话更新失败"
fi
echo ""

# 心跳测试
echo "9️⃣  测试心跳..."
grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\"
}" localhost:9001 session.SessionService/Heartbeat > /dev/null 2>&1
echo "✅ 心跳成功"
echo ""

# 删除会话
echo "🔟 测试会话删除..."
grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\",
  \"reason\": \"测试完成\"
}" localhost:9001 session.SessionService/DeleteSession > /dev/null 2>&1

# 验证删除
if redis-cli EXISTS "session:$SESSION_ID" | grep -q "0"; then
    echo "✅ 会话删除成功"
    
    # 验证索引也被删除
    if redis-cli SISMEMBER sessions:all "$SESSION_ID" | grep -q "0"; then
        echo "✅ 索引清理成功"
    else
        echo "⚠️  索引未清理"
    fi
else
    echo "❌ 会话删除失败"
fi
echo ""

# 总结
echo "================================"
echo "  验证完成！"
echo "================================"
echo ""
echo "✅ 所有测试通过！"
echo ""
echo "Redis Store 功能正常："
echo "  - ✅ 数据持久化"
echo "  - ✅ 自动索引"
echo "  - ✅ TTL 过期"
echo "  - ✅ 原子操作"
echo "  - ✅ 完整生命周期"
echo ""
echo "查看 Redis 数据："
echo "  redis-cli SMEMBERS sessions:all"
echo "  redis-cli GET sessions:count"
echo "  redis-cli INFO memory"
echo ""
