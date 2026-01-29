package statesync

import (
	"context"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"go.uber.org/zap"
)

// ConflictResolver 冲突解决器接口
type ConflictResolver interface {
	// Resolve 解决冲突
	// 返回: 解决后的操作, 错误
	Resolve(ctx context.Context, ops []*Operation) (*Operation, error)

	// GetStrategy 获取解决策略
	GetStrategy() ConflictResolution
}

// LWWConflictResolver Last-Write-Wins冲突解决器
// 使用时间戳选择最新的操作
type LWWConflictResolver struct {
	logger *zap.Logger
}

// NewLWWConflictResolver 创建LWW冲突解决器
func NewLWWConflictResolver(logger *zap.Logger) *LWWConflictResolver {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LWWConflictResolver{
		logger: logger,
	}
}

// Resolve 解决冲突 - 选择时间戳最新的操作
func (r *LWWConflictResolver) Resolve(ctx context.Context, ops []*Operation) (*Operation, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations to resolve")
	}

	if len(ops) == 1 {
		return ops[0], nil
	}

	// 找到时间戳最新的操作
	latest := ops[0]
	for _, op := range ops[1:] {
		if op.Timestamp.After(latest.Timestamp) {
			latest = op
		} else if op.Timestamp.Equal(latest.Timestamp) {
			// 如果时间戳相同，使用UUIDv7 (其中包含时间戳)
			if compareUUID(op.ID, latest.ID) > 0 {
				latest = op
			}
		}
	}

	r.logger.Debug("LWW conflict resolved",
		zap.String("selected_op_id", latest.ID.String()),
		zap.String("selected_user", latest.UserID),
		zap.Time("timestamp", latest.Timestamp),
		zap.Int("total_ops", len(ops)),
	)

	return latest, nil
}

// GetStrategy 获取策略
func (r *LWWConflictResolver) GetStrategy() ConflictResolution {
	return ConflictResolutionLWW
}

// ManualConflictResolver 手动冲突解决器
// 需要人工介入解决冲突
type ManualConflictResolver struct {
	logger *zap.Logger
}

// NewManualConflictResolver 创建手动冲突解决器
func NewManualConflictResolver(logger *zap.Logger) *ManualConflictResolver {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ManualConflictResolver{
		logger: logger,
	}
}

// Resolve 标记为需要手动解决
func (r *ManualConflictResolver) Resolve(ctx context.Context, ops []*Operation) (*Operation, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations to resolve")
	}

	r.logger.Info("Manual conflict resolution required",
		zap.Int("conflicting_ops", len(ops)),
	)

	// 返回第一个操作，但标记为需要手动解决
	// 实际应用中，应该将冲突记录到数据库，等待人工处理
	return nil, fmt.Errorf("manual resolution required for %d conflicting operations", len(ops))
}

// GetStrategy 获取策略
func (r *ManualConflictResolver) GetStrategy() ConflictResolution {
	return ConflictResolutionManual
}

// MergeConflictResolver 合并冲突解决器
// 尝试自动合并不冲突的部分
type MergeConflictResolver struct {
	logger *zap.Logger
}

// NewMergeConflictResolver 创建合并冲突解决器
func NewMergeConflictResolver(logger *zap.Logger) *MergeConflictResolver {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MergeConflictResolver{
		logger: logger,
	}
}

// Resolve 尝试合并操作
func (r *MergeConflictResolver) Resolve(ctx context.Context, ops []*Operation) (*Operation, error) {
	if len(ops) == 0 {
		return nil, fmt.Errorf("no operations to resolve")
	}

	if len(ops) == 1 {
		return ops[0], nil
	}

	// 按时间戳排序
	sortedOps := make([]*Operation, len(ops))
	copy(sortedOps, ops)
	sortOperationsByTimestamp(sortedOps)

	// 尝试合并操作
	// 这里是简化实现，实际应该根据操作类型进行智能合并
	// 例如：
	// - 如果操作在不同的对象上，可以并行应用
	// - 如果操作在同一对象的不同属性上，可以合并
	// - 如果操作完全冲突，则需要选择一个

	merged := sortedOps[0]

	r.logger.Debug("Merge conflict resolution",
		zap.Int("total_ops", len(ops)),
		zap.String("merged_op_id", merged.ID.String()),
	)

	// 实际实现应该更复杂，这里返回最早的操作作为基础
	return merged, nil
}

// GetStrategy 获取策略
func (r *MergeConflictResolver) GetStrategy() ConflictResolution {
	return ConflictResolutionMerge
}

// ConflictDetector 冲突检测器
type ConflictDetector struct {
	logger *zap.Logger
}

// NewConflictDetector 创建冲突检测器
func NewConflictDetector(logger *zap.Logger) *ConflictDetector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ConflictDetector{
		logger: logger,
	}
}

// DetectConflicts 检测操作中的冲突
// 检查多个并发操作是否存在冲突
func (d *ConflictDetector) DetectConflicts(ops []*Operation, doc *Document) ([]*Conflict, error) {
	if len(ops) <= 1 {
		return nil, nil
	}

	conflicts := make([]*Conflict, 0)

	// 按文档ID分组
	opsByDoc := make(map[guuid.UUID][]*Operation)
	for _, op := range ops {
		opsByDoc[op.DocID] = append(opsByDoc[op.DocID], op)
	}

	// 检测每个文档的冲突
	for docID, docOps := range opsByDoc {
		if len(docOps) <= 1 {
			continue
		}

		// 检查是否有版本冲突
		hasConflict := false
		baseVersion := docOps[0].PrevVersion

		for _, op := range docOps {
			if op.PrevVersion != baseVersion {
				hasConflict = true
				break
			}
		}

		if hasConflict {
			conflictID, _ := guuid.NewV7()
			conflict := &Conflict{
				ID:          conflictID,
				DocID:       docID,
				Ops:         docOps,
				Resolution:  ConflictResolutionLWW, // 默认使用LWW
				Description: fmt.Sprintf("Version conflict detected: %d operations on same version", len(docOps)),
			}
			conflicts = append(conflicts, conflict)

			d.logger.Warn("Conflict detected",
				zap.String("conflict_id", conflict.ID.String()),
				zap.String("doc_id", docID.String()),
				zap.Int("conflicting_ops", len(docOps)),
			)
		}
	}

	return conflicts, nil
}

// IsConflicting 判断两个操作是否冲突
func (d *ConflictDetector) IsConflicting(op1, op2 *Operation) bool {
	// 不同文档的操作不冲突
	if op1.DocID != op2.DocID {
		return false
	}

	// 检查版本号
	// 如果两个操作基于同一个版本，则可能冲突
	if op1.PrevVersion == op2.PrevVersion {
		return true
	}

	return false
}

// ResolveConflict 解决单个冲突
func ResolveConflict(ctx context.Context, conflict *Conflict, resolver ConflictResolver, userID string) error {
	if conflict == nil {
		return fmt.Errorf("conflict is nil")
	}

	// 使用解决器解决冲突
	resolvedOp, err := resolver.Resolve(ctx, conflict.Ops)
	if err != nil {
		return fmt.Errorf("failed to resolve conflict: %w", err)
	}

	// 更新冲突记录
	conflict.ResolvedBy = userID
	conflict.ResolvedOp = resolvedOp
	conflict.ResolvedAt = time.Now()
	conflict.Resolution = resolver.GetStrategy()

	return nil
}

// ==================== 辅助函数 ====================

// compareUUID 比较两个UUID (用于UUIDv7的时间排序)
func compareUUID(a, b guuid.UUID) int {
	for i := 0; i < 16; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// sortOperationsByTimestamp 按时间戳排序操作
func sortOperationsByTimestamp(ops []*Operation) {
	// 简单的冒泡排序
	for i := 0; i < len(ops); i++ {
		for j := i + 1; j < len(ops); j++ {
			if ops[j].Timestamp.Before(ops[i].Timestamp) {
				ops[i], ops[j] = ops[j], ops[i]
			}
		}
	}
}
