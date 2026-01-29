package statesync

import (
	"time"

	guuid "github.com/Lzww0608/GUUID"
)

// DocumentType 文档类型
type DocumentType string

const (
	DocumentTypeWhiteboard DocumentType = "whiteboard" // 白板
	DocumentTypeText       DocumentType = "text"       // 文本文档
	DocumentTypeCanvas     DocumentType = "canvas"     // 画布
	DocumentTypeSheet      DocumentType = "sheet"      // 表格
)

// DocumentState 文档状态
type DocumentState string

const (
	DocumentStateActive   DocumentState = "active"   // 活跃
	DocumentStateArchived DocumentState = "archived" // 已归档
	DocumentStateDeleted  DocumentState = "deleted"  // 已删除
)

// Document 协作文档/对象模型
type Document struct {
	ID          guuid.UUID    `json:"id"`           // 文档ID (UUIDv7)
	Name        string        `json:"name"`         // 文档名称
	Type        DocumentType  `json:"type"`         // 文档类型
	State       DocumentState `json:"state"`        // 文档状态
	Version     uint64        `json:"version"`      // 版本号 (单调递增)
	Content     []byte        `json:"content"`      // 当前内容 (序列化)
	CreatedBy   string        `json:"created_by"`   // 创建者UserID
	CreatedAt   time.Time     `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time     `json:"updated_at"`   // 最后更新时间
	UpdatedBy   string        `json:"updated_by"`   // 最后更新者
	ActiveUsers []string      `json:"active_users"` // 活跃用户列表
	Metadata    Metadata      `json:"metadata"`     // 元数据
}

// Metadata 文档元数据
type Metadata struct {
	Tags        []string          `json:"tags"`         // 标签
	Description string            `json:"description"`  // 描述
	Properties  map[string]string `json:"properties"`   // 自定义属性
	Permissions Permissions       `json:"permissions"`  // 权限设置
}

// Permissions 权限设置
type Permissions struct {
	Owner   string   `json:"owner"`   // 拥有者
	Editors []string `json:"editors"` // 编辑者列表
	Viewers []string `json:"viewers"` // 查看者列表
	Public  bool     `json:"public"`  // 是否公开
}

// OperationType 操作类型
type OperationType string

const (
	OperationTypeCreate OperationType = "create" // 创建
	OperationTypeUpdate OperationType = "update" // 更新
	OperationTypeDelete OperationType = "delete" // 删除
	OperationTypeMove   OperationType = "move"   // 移动
	OperationTypeResize OperationType = "resize" // 调整大小
	OperationTypeStyle  OperationType = "style"  // 样式变更
	OperationTypeText   OperationType = "text"   // 文本编辑
)

// OperationStatus 操作状态
type OperationStatus string

const (
	OperationStatusPending   OperationStatus = "pending"   // 待处理
	OperationStatusApplied   OperationStatus = "applied"   // 已应用
	OperationStatusConflict  OperationStatus = "conflict"  // 冲突
	OperationStatusRejected  OperationStatus = "rejected"  // 已拒绝
	OperationStatusResolved  OperationStatus = "resolved"  // 已解决
)

// Operation 操作日志
type Operation struct {
	ID          guuid.UUID      `json:"id"`           // 操作ID (UUIDv7)
	DocID       guuid.UUID      `json:"doc_id"`       // 文档ID
	UserID      string          `json:"user_id"`      // 用户ID
	SessionID   guuid.UUID      `json:"session_id"`   // 会话ID
	Type        OperationType   `json:"type"`         // 操作类型
	Data        []byte          `json:"data"`         // 操作数据 (序列化)
	Timestamp   time.Time       `json:"timestamp"`    // 时间戳
	Version     uint64          `json:"version"`      // 操作版本号
	PrevVersion uint64          `json:"prev_version"` // 前一个版本号
	Status      OperationStatus `json:"status"`       // 操作状态
	ClientID    string          `json:"client_id"`    // 客户端ID
	Metadata    OpMetadata      `json:"metadata"`     // 操作元数据
}

// OpMetadata 操作元数据
type OpMetadata struct {
	IP        string            `json:"ip"`         // 客户端IP
	UserAgent string            `json:"user_agent"` // 用户代理
	Platform  string            `json:"platform"`   // 平台
	Extra     map[string]string `json:"extra"`      // 额外信息
}

// ConflictResolution 冲突解决策略
type ConflictResolution string

const (
	ConflictResolutionLWW    ConflictResolution = "lww"    // Last-Write-Wins
	ConflictResolutionManual ConflictResolution = "manual" // 手动解决
	ConflictResolutionMerge  ConflictResolution = "merge"  // 自动合并
)

// Conflict 冲突记录
type Conflict struct {
	ID          guuid.UUID         `json:"id"`          // 冲突ID
	DocID       guuid.UUID         `json:"doc_id"`      // 文档ID
	Ops         []*Operation       `json:"ops"`         // 冲突的操作列表
	Resolution  ConflictResolution `json:"resolution"`  // 解决策略
	ResolvedBy  string             `json:"resolved_by"` // 解决者
	ResolvedOp  *Operation         `json:"resolved_op"` // 解决后的操作
	ResolvedAt  time.Time          `json:"resolved_at"` // 解决时间
	Description string             `json:"description"` // 冲突描述
}

// Subscriber 订阅者信息
type Subscriber struct {
	ID        string      `json:"id"`         // 订阅者ID
	UserID    string      `json:"user_id"`    // 用户ID
	SessionID guuid.UUID  `json:"session_id"` // 会话ID
	DocID     guuid.UUID  `json:"doc_id"`     // 文档ID
	Channel   chan *Event `json:"-"`          // 事件通道 (不序列化)
	CreatedAt time.Time   `json:"created_at"` // 创建时间
	Active    bool        `json:"active"`     // 是否活跃
}

// EventType 事件类型
type EventType string

const (
	EventTypeOperationApplied EventType = "operation_applied" // 操作已应用
	EventTypeDocumentUpdated  EventType = "document_updated"  // 文档已更新
	EventTypeUserJoined       EventType = "user_joined"       // 用户加入
	EventTypeUserLeft         EventType = "user_left"         // 用户离开
	EventTypeConflictDetected EventType = "conflict_detected" // 检测到冲突
	EventTypeConflictResolved EventType = "conflict_resolved" // 冲突已解决
	EventTypeLockAcquired     EventType = "lock_acquired"     // 获取锁
	EventTypeLockReleased     EventType = "lock_released"     // 释放锁
)

// Event 事件
type Event struct {
	ID        guuid.UUID  `json:"id"`        // 事件ID
	Type      EventType   `json:"type"`      // 事件类型
	DocID     guuid.UUID  `json:"doc_id"`    // 文档ID
	UserID    string      `json:"user_id"`   // 用户ID
	Operation *Operation  `json:"operation"` // 关联的操作
	Document  *Document   `json:"document"`  // 关联的文档
	Conflict  *Conflict   `json:"conflict"`  // 关联的冲突
	Timestamp time.Time   `json:"timestamp"` // 时间戳
	Data      interface{} `json:"data"`      // 事件数据
}

// Lock 文档锁
type Lock struct {
	ID         guuid.UUID `json:"id"`          // 锁ID
	DocID      guuid.UUID `json:"doc_id"`      // 文档ID
	UserID     string     `json:"user_id"`     // 持有者UserID
	SessionID  guuid.UUID `json:"session_id"`  // 会话ID
	AcquiredAt time.Time  `json:"acquired_at"` // 获取时间
	ExpiresAt  time.Time  `json:"expires_at"`  // 过期时间
	Active     bool       `json:"active"`      // 是否活跃
}

// DocumentFilter 文档过滤器
type DocumentFilter struct {
	Type      *DocumentType  `json:"type"`       // 按类型过滤
	State     *DocumentState `json:"state"`      // 按状态过滤
	CreatedBy *string        `json:"created_by"` // 按创建者过滤
	UserID    *string        `json:"user_id"`    // 按用户ID过滤 (查找用户有权限的文档)
	Offset    int            `json:"offset"`     // 分页偏移
	Limit     int            `json:"limit"`      // 分页限制
}

// OperationFilter 操作过滤器
type OperationFilter struct {
	DocID      *guuid.UUID      `json:"doc_id"`      // 按文档ID过滤
	UserID     *string          `json:"user_id"`     // 按用户ID过滤
	Type       *OperationType   `json:"type"`        // 按操作类型过滤
	Status     *OperationStatus `json:"status"`      // 按状态过滤
	FromTime   *time.Time       `json:"from_time"`   // 起始时间
	ToTime     *time.Time       `json:"to_time"`     // 结束时间
	MinVersion *uint64          `json:"min_version"` // 最小版本号
	MaxVersion *uint64          `json:"max_version"` // 最大版本号
	Offset     int              `json:"offset"`      // 分页偏移
	Limit      int              `json:"limit"`       // 分页限制
}

// Stats 统计信息
type Stats struct {
	TotalDocuments    int64     `json:"total_documents"`    // 文档总数
	ActiveDocuments   int64     `json:"active_documents"`   // 活跃文档数
	ArchivedDocuments int64     `json:"archived_documents"` // 归档文档数
	TotalOperations   int64     `json:"total_operations"`   // 操作总数
	TotalConflicts    int64     `json:"total_conflicts"`    // 冲突总数
	ResolvedConflicts int64     `json:"resolved_conflicts"` // 已解决冲突数
	ActiveSubscribers int64     `json:"active_subscribers"` // 活跃订阅者数
	ActiveLocks       int64     `json:"active_locks"`       // 活跃锁数
	LastUpdated       time.Time `json:"last_updated"`       // 最后更新时间
}
