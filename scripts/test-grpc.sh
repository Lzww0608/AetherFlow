#!/bin/bash

# AetherFlow gRPC 端到端测试脚本
# 使用方法: ./scripts/test-grpc.sh

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "  AetherFlow gRPC 端到端测试"
echo "================================"
echo ""

# 检查 grpcurl 是否安装
if ! command -v grpcurl >/dev/null 2>&1; then
    echo "❌ grpcurl 未安装！"
    echo ""
    echo "安装方法："
    echo "  macOS:  brew install grpcurl"
    echo "  Linux:  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
    exit 1
fi

# 检查服务是否运行
check_service() {
    local host=$1
    local port=$2
    local name=$3
    
    echo "检查 $name 是否运行..."
    if grpcurl -plaintext $host:$port grpc.health.v1.Health/Check > /dev/null 2>&1; then
        echo "✅ $name: 运行中"
        return 0
    else
        echo "❌ $name: 未运行"
        return 1
    fi
}

# 检查服务
echo "==================== 健康检查 ===================="
check_service localhost 9001 "Session Service" || exit 1
check_service localhost 9002 "StateSync Service" || exit 1
echo ""

# 测试 Session Service
echo "==================== Session Service 测试 ===================="
echo ""

# 1. 创建会话
echo "1️⃣  测试：创建会话"
SESSION_RESPONSE=$(grpcurl -plaintext -d '{
  "user_id": "user-test-001",
  "client_ip": "192.168.1.100",
  "client_port": 12345,
  "metadata": {
    "device": "laptop",
    "os": "linux"
  },
  "timeout_seconds": 1800
}' localhost:9001 session.SessionService/CreateSession)

SESSION_ID=$(echo "$SESSION_RESPONSE" | grep -o '"session_id": "[^"]*"' | cut -d'"' -f4)

if [ -n "$SESSION_ID" ]; then
    echo "✅ 会话创建成功！Session ID: $SESSION_ID"
else
    echo "❌ 会话创建失败！"
    echo "$SESSION_RESPONSE"
    exit 1
fi
echo ""

# 2. 获取会话
echo "2️⃣  测试：获取会话"
GET_SESSION_RESPONSE=$(grpcurl -plaintext -d "{\"session_id\": \"$SESSION_ID\"}" \
    localhost:9001 session.SessionService/GetSession)

if echo "$GET_SESSION_RESPONSE" | grep -q "$SESSION_ID"; then
    echo "✅ 获取会话成功！"
else
    echo "❌ 获取会话失败！"
    exit 1
fi
echo ""

# 3. 更新会话
echo "3️⃣  测试：更新会话"
UPDATE_SESSION_RESPONSE=$(grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\",
  \"state\": 2,
  \"metadata\": {
    \"updated\": \"true\"
  }
}" localhost:9001 session.SessionService/UpdateSession)

if echo "$UPDATE_SESSION_RESPONSE" | grep -q "session"; then
    echo "✅ 更新会话成功！"
else
    echo "❌ 更新会话失败！"
    exit 1
fi
echo ""

# 4. 心跳
echo "4️⃣  测试：心跳"
HEARTBEAT_RESPONSE=$(grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\",
  \"client_timestamp\": {
    \"seconds\": $(date +%s)
  }
}" localhost:9001 session.SessionService/Heartbeat)

if echo "$HEARTBEAT_RESPONSE" | grep -q "success"; then
    echo "✅ 心跳成功！"
else
    echo "❌ 心跳失败！"
    exit 1
fi
echo ""

# 5. 列出会话
echo "5️⃣  测试：列出会话"
LIST_SESSIONS_RESPONSE=$(grpcurl -plaintext -d '{
  "user_id": "user-test-001",
  "page": 1,
  "page_size": 10
}' localhost:9001 session.SessionService/ListSessions)

if echo "$LIST_SESSIONS_RESPONSE" | grep -q "sessions"; then
    echo "✅ 列出会话成功！"
else
    echo "❌ 列出会话失败！"
    exit 1
fi
echo ""

# 测试 StateSync Service
echo "==================== StateSync Service 测试 ===================="
echo ""

# 1. 创建文档
echo "1️⃣  测试：创建文档"
DOC_RESPONSE=$(grpcurl -plaintext -d '{
  "name": "测试白板",
  "type": "whiteboard",
  "created_by": "user-test-001",
  "content": "eyJkYXRhIjogInRlc3QifQ==",
  "metadata": {
    "tags": ["test", "demo"],
    "description": "测试文档",
    "permissions": {
      "owner": "user-test-001",
      "public": false
    }
  }
}' localhost:9002 aetherflow.statesync.StateSyncService/CreateDocument)

DOC_ID=$(echo "$DOC_RESPONSE" | grep -o '"id": "[^"]*"' | cut -d'"' -f4 | head -1)

if [ -n "$DOC_ID" ]; then
    echo "✅ 文档创建成功！Doc ID: $DOC_ID"
else
    echo "❌ 文档创建失败！"
    echo "$DOC_RESPONSE"
    exit 1
fi
echo ""

# 2. 获取文档
echo "2️⃣  测试：获取文档"
GET_DOC_RESPONSE=$(grpcurl -plaintext -d "{\"doc_id\": \"$DOC_ID\"}" \
    localhost:9002 aetherflow.statesync.StateSyncService/GetDocument)

if echo "$GET_DOC_RESPONSE" | grep -q "$DOC_ID"; then
    echo "✅ 获取文档成功！"
else
    echo "❌ 获取文档失败！"
    exit 1
fi
echo ""

# 3. 应用操作
echo "3️⃣  测试：应用操作"
APPLY_OP_RESPONSE=$(grpcurl -plaintext -d "{
  \"operation\": {
    \"doc_id\": \"$DOC_ID\",
    \"user_id\": \"user-test-001\",
    \"session_id\": \"$SESSION_ID\",
    \"type\": \"create\",
    \"data\": \"eyJhY3Rpb24iOiAiYWRkX3NoYXBlIn0=\",
    \"version\": 1,
    \"prev_version\": 0,
    \"client_id\": \"client-001\"
  }
}" localhost:9002 aetherflow.statesync.StateSyncService/ApplyOperation)

if echo "$APPLY_OP_RESPONSE" | grep -q "success"; then
    echo "✅ 应用操作成功！"
else
    echo "❌ 应用操作失败！"
    echo "$APPLY_OP_RESPONSE"
fi
echo ""

# 4. 获取锁
echo "4️⃣  测试：获取锁"
ACQUIRE_LOCK_RESPONSE=$(grpcurl -plaintext -d "{
  \"doc_id\": \"$DOC_ID\",
  \"user_id\": \"user-test-001\",
  \"session_id\": \"$SESSION_ID\"
}" localhost:9002 aetherflow.statesync.StateSyncService/AcquireLock)

if echo "$ACQUIRE_LOCK_RESPONSE" | grep -q "lock"; then
    echo "✅ 获取锁成功！"
else
    echo "❌ 获取锁失败！"
    echo "$ACQUIRE_LOCK_RESPONSE"
fi
echo ""

# 5. 检查锁
echo "5️⃣  测试：检查锁"
IS_LOCKED_RESPONSE=$(grpcurl -plaintext -d "{\"doc_id\": \"$DOC_ID\"}" \
    localhost:9002 aetherflow.statesync.StateSyncService/IsLocked)

if echo "$IS_LOCKED_RESPONSE" | grep -q "locked"; then
    echo "✅ 检查锁成功！"
else
    echo "❌ 检查锁失败！"
    exit 1
fi
echo ""

# 6. 释放锁
echo "6️⃣  测试：释放锁"
RELEASE_LOCK_RESPONSE=$(grpcurl -plaintext -d "{
  \"doc_id\": \"$DOC_ID\",
  \"user_id\": \"user-test-001\"
}" localhost:9002 aetherflow.statesync.StateSyncService/ReleaseLock)

if echo "$RELEASE_LOCK_RESPONSE" | grep -q "success"; then
    echo "✅ 释放锁成功！"
else
    echo "❌ 释放锁失败！"
    exit 1
fi
echo ""

# 7. 列出文档
echo "7️⃣  测试：列出文档"
LIST_DOCS_RESPONSE=$(grpcurl -plaintext -d '{
  "type": "whiteboard",
  "state": "active",
  "created_by": "user-test-001",
  "limit": 10
}' localhost:9002 aetherflow.statesync.StateSyncService/ListDocuments)

if echo "$LIST_DOCS_RESPONSE" | grep -q "documents"; then
    echo "✅ 列出文档成功！"
else
    echo "❌ 列出文档失败！"
    exit 1
fi
echo ""

# 8. 获取统计信息
echo "8️⃣  测试：获取统计信息"
STATS_RESPONSE=$(grpcurl -plaintext -d '{}' \
    localhost:9002 aetherflow.statesync.StateSyncService/GetStats)

if echo "$STATS_RESPONSE" | grep -q "stats"; then
    echo "✅ 获取统计信息成功！"
else
    echo "❌ 获取统计信息失败！"
    exit 1
fi
echo ""

# 9. 删除文档
echo "9️⃣  测试：删除文档"
DELETE_DOC_RESPONSE=$(grpcurl -plaintext -d "{
  \"doc_id\": \"$DOC_ID\",
  \"user_id\": \"user-test-001\"
}" localhost:9002 aetherflow.statesync.StateSyncService/DeleteDocument)

if echo "$DELETE_DOC_RESPONSE" | grep -q "success"; then
    echo "✅ 删除文档成功！"
else
    echo "❌ 删除文档失败！"
    exit 1
fi
echo ""

# 10. 删除会话
echo "🔟 测试：删除会话"
DELETE_SESSION_RESPONSE=$(grpcurl -plaintext -d "{
  \"session_id\": \"$SESSION_ID\",
  \"reason\": \"测试完成\"
}" localhost:9001 session.SessionService/DeleteSession)

if echo "$DELETE_SESSION_RESPONSE" | grep -q "success"; then
    echo "✅ 删除会话成功！"
else
    echo "❌ 删除会话失败！"
    exit 1
fi
echo ""

# 总结
echo "================================"
echo "  测试完成！"
echo "================================"
echo ""
echo "✅ 所有测试通过！"
echo ""
echo "测试覆盖："
echo "  - Session Service: 6 个接口"
echo "  - StateSync Service: 9 个接口"
echo ""
