package statesync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	guuid "github.com/Lzww0608/GUUID"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresStore PostgreSQL implementation of Store interface
type PostgresStore struct {
	db     *sql.DB
	logger *zap.Logger
}

// PostgresStoreConfig PostgreSQL store configuration
type PostgresStoreConfig struct {
	DB     *sql.DB
	Logger *zap.Logger
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(config *PostgresStoreConfig) (*PostgresStore, error) {
	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	store := &PostgresStore{
		db:     config.DB,
		logger: config.Logger,
	}

	return store, nil
}

// ==================== 文档管理 ====================

// CreateDocument creates a new document
func (s *PostgresStore) CreateDocument(ctx context.Context, doc *Document) error {
	query := `
		INSERT INTO documents (
			id, name, type, state, version, content,
			created_by, created_at, updated_at, updated_by,
			active_users, tags, description, properties,
			owner, editors, viewers, public
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18
		)`

	properties, err := json.Marshal(doc.Metadata.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		doc.ID.String(),
		doc.Name,
		string(doc.Type),
		string(doc.State),
		doc.Version,
		doc.Content,
		doc.CreatedBy,
		doc.CreatedAt,
		doc.UpdatedAt,
		doc.UpdatedBy,
		pq.Array(doc.ActiveUsers),
		pq.Array(doc.Metadata.Tags),
		doc.Metadata.Description,
		properties,
		doc.Metadata.Permissions.Owner,
		pq.Array(doc.Metadata.Permissions.Editors),
		pq.Array(doc.Metadata.Permissions.Viewers),
		doc.Metadata.Permissions.Public,
	)

	if err != nil {
		s.logger.Error("Failed to create document", zap.Error(err))
		return fmt.Errorf("failed to create document: %w", err)
	}

	return nil
}

// GetDocument retrieves a document by ID
func (s *PostgresStore) GetDocument(ctx context.Context, docID guuid.UUID) (*Document, error) {
	query := `
		SELECT 
			id, name, type, state, version, content,
			created_by, created_at, updated_at, updated_by,
			active_users, tags, description, properties,
			owner, editors, viewers, public
		FROM documents
		WHERE id = $1 AND state != 'deleted'`

	var doc Document
	var properties []byte
	var activeUsers, tags, editors, viewers pq.StringArray

	err := s.db.QueryRowContext(ctx, query, docID.String()).Scan(
		&doc.ID,
		&doc.Name,
		&doc.Type,
		&doc.State,
		&doc.Version,
		&doc.Content,
		&doc.CreatedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
		&doc.UpdatedBy,
		&activeUsers,
		&tags,
		&doc.Metadata.Description,
		&properties,
		&doc.Metadata.Permissions.Owner,
		&editors,
		&viewers,
		&doc.Metadata.Permissions.Public,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found: %s", docID.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	// Convert arrays
	doc.ActiveUsers = []string(activeUsers)
	doc.Metadata.Tags = []string(tags)
	doc.Metadata.Permissions.Editors = []string(editors)
	doc.Metadata.Permissions.Viewers = []string(viewers)

	// Unmarshal properties
	if len(properties) > 0 {
		if err := json.Unmarshal(properties, &doc.Metadata.Properties); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	}

	return &doc, nil
}

// UpdateDocument updates an existing document
func (s *PostgresStore) UpdateDocument(ctx context.Context, doc *Document) error {
	query := `
		UPDATE documents SET
			name = $2,
			type = $3,
			state = $4,
			version = $5,
			content = $6,
			updated_by = $7,
			active_users = $8,
			tags = $9,
			description = $10,
			properties = $11,
			owner = $12,
			editors = $13,
			viewers = $14,
			public = $15
		WHERE id = $1`

	properties, err := json.Marshal(doc.Metadata.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	result, err := s.db.ExecContext(ctx, query,
		doc.ID.String(),
		doc.Name,
		string(doc.Type),
		string(doc.State),
		doc.Version,
		doc.Content,
		doc.UpdatedBy,
		pq.Array(doc.ActiveUsers),
		pq.Array(doc.Metadata.Tags),
		doc.Metadata.Description,
		properties,
		doc.Metadata.Permissions.Owner,
		pq.Array(doc.Metadata.Permissions.Editors),
		pq.Array(doc.Metadata.Permissions.Viewers),
		doc.Metadata.Permissions.Public,
	)

	if err != nil {
		return fmt.Errorf("failed to update document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("document not found: %s", doc.ID.String())
	}

	return nil
}

// DeleteDocument soft deletes a document
func (s *PostgresStore) DeleteDocument(ctx context.Context, docID guuid.UUID) error {
	query := `UPDATE documents SET state = 'deleted', updated_at = CURRENT_TIMESTAMP WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, docID.String())
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("document not found: %s", docID.String())
	}

	return nil
}

// ListDocuments lists documents with filtering
func (s *PostgresStore) ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, int, error) {
	// Build query with filters
	query := `
		SELECT 
			id, name, type, state, version, content,
			created_by, created_at, updated_at, updated_by,
			active_users, tags, description, properties,
			owner, editors, viewers, public
		FROM documents
		WHERE 1=1`
	
	countQuery := `SELECT COUNT(*) FROM documents WHERE 1=1`
	
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if filter != nil {
		if filter.Type != nil {
			query += fmt.Sprintf(" AND type = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND type = $%d", argIndex)
			args = append(args, string(*filter.Type))
			argIndex++
		}
		if filter.State != nil {
			query += fmt.Sprintf(" AND state = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND state = $%d", argIndex)
			args = append(args, string(*filter.State))
			argIndex++
		}
		if filter.CreatedBy != nil {
			query += fmt.Sprintf(" AND created_by = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND created_by = $%d", argIndex)
			args = append(args, *filter.CreatedBy)
			argIndex++
		}
	}

	// Order and pagination
	query += " ORDER BY created_at DESC"
	
	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, filter.Limit)
			argIndex++
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, filter.Offset)
			argIndex++
		}
	}

	// Get total count
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args[:len(args)-2]...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Query documents
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	documents := []*Document{}
	for rows.Next() {
		var doc Document
		var properties []byte
		var activeUsers, tags, editors, viewers pq.StringArray

		err := rows.Scan(
			&doc.ID,
			&doc.Name,
			&doc.Type,
			&doc.State,
			&doc.Version,
			&doc.Content,
			&doc.CreatedBy,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&doc.UpdatedBy,
			&activeUsers,
			&tags,
			&doc.Metadata.Description,
			&properties,
			&doc.Metadata.Permissions.Owner,
			&editors,
			&viewers,
			&doc.Metadata.Permissions.Public,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document: %w", err)
		}

		// Convert arrays
		doc.ActiveUsers = []string(activeUsers)
		doc.Metadata.Tags = []string(tags)
		doc.Metadata.Permissions.Editors = []string(editors)
		doc.Metadata.Permissions.Viewers = []string(viewers)

		// Unmarshal properties
		if len(properties) > 0 {
			if err := json.Unmarshal(properties, &doc.Metadata.Properties); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal properties: %w", err)
			}
		}

		documents = append(documents, &doc)
	}

	return documents, total, nil
}

// GetDocumentsByUser gets documents accessible by user
func (s *PostgresStore) GetDocumentsByUser(ctx context.Context, userID string) ([]*Document, error) {
	query := `
		SELECT 
			id, name, type, state, version, content,
			created_by, created_at, updated_at, updated_by,
			active_users, tags, description, properties,
			owner, editors, viewers, public
		FROM documents
		WHERE state != 'deleted'
		  AND (
		    created_by = $1
		    OR owner = $1
		    OR $1 = ANY(editors)
		    OR $1 = ANY(viewers)
		    OR public = TRUE
		  )
		ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get documents by user: %w", err)
	}
	defer rows.Close()

	documents := []*Document{}
	for rows.Next() {
		var doc Document
		var properties []byte
		var activeUsers, tags, editors, viewers pq.StringArray

		err := rows.Scan(
			&doc.ID,
			&doc.Name,
			&doc.Type,
			&doc.State,
			&doc.Version,
			&doc.Content,
			&doc.CreatedBy,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&doc.UpdatedBy,
			&activeUsers,
			&tags,
			&doc.Metadata.Description,
			&properties,
			&doc.Metadata.Permissions.Owner,
			&editors,
			&viewers,
			&doc.Metadata.Permissions.Public,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		doc.ActiveUsers = []string(activeUsers)
		doc.Metadata.Tags = []string(tags)
		doc.Metadata.Permissions.Editors = []string(editors)
		doc.Metadata.Permissions.Viewers = []string(viewers)

		if len(properties) > 0 {
			json.Unmarshal(properties, &doc.Metadata.Properties)
		}

		documents = append(documents, &doc)
	}

	return documents, nil
}

// UpdateDocumentVersion atomically updates document version
func (s *PostgresStore) UpdateDocumentVersion(ctx context.Context, docID guuid.UUID, oldVersion, newVersion uint64, content []byte) error {
	query := `SELECT atomic_update_document_version($1, $2, $3, $4, $5)`

	var success bool
	err := s.db.QueryRowContext(ctx, query,
		docID.String(),
		oldVersion,
		newVersion,
		content,
		"system", // updated_by
	).Scan(&success)

	if err != nil {
		return fmt.Errorf("failed to update document version: %w", err)
	}

	if !success {
		return fmt.Errorf("version conflict: expected %d, document may have been updated", oldVersion)
	}

	return nil
}

// AddActiveUser adds a user to active users list
func (s *PostgresStore) AddActiveUser(ctx context.Context, docID guuid.UUID, userID string) error {
	query := `SELECT add_active_user($1, $2)`
	_, err := s.db.ExecContext(ctx, query, docID.String(), userID)
	if err != nil {
		return fmt.Errorf("failed to add active user: %w", err)
	}
	return nil
}

// RemoveActiveUser removes a user from active users list
func (s *PostgresStore) RemoveActiveUser(ctx context.Context, docID guuid.UUID, userID string) error {
	query := `SELECT remove_active_user($1, $2)`
	_, err := s.db.ExecContext(ctx, query, docID.String(), userID)
	if err != nil {
		return fmt.Errorf("failed to remove active user: %w", err)
	}
	return nil
}

// ==================== 操作管理 ====================

// CreateOperation creates a new operation
func (s *PostgresStore) CreateOperation(ctx context.Context, op *Operation) error {
	query := `
		INSERT INTO operations (
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15
		)`

	extra, err := json.Marshal(op.Metadata.Extra)
	if err != nil {
		return fmt.Errorf("failed to marshal extra metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, query,
		op.ID.String(),
		op.DocID.String(),
		op.UserID,
		op.SessionID.String(),
		string(op.Type),
		op.Data,
		op.Timestamp,
		op.Version,
		op.PrevVersion,
		string(op.Status),
		op.ClientID,
		op.Metadata.IP,
		op.Metadata.UserAgent,
		op.Metadata.Platform,
		extra,
	)

	if err != nil {
		return fmt.Errorf("failed to create operation: %w", err)
	}

	return nil
}

// GetOperation retrieves an operation by ID
func (s *PostgresStore) GetOperation(ctx context.Context, opID guuid.UUID) (*Operation, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		FROM operations
		WHERE id = $1`

	var op Operation
	var extra []byte

	err := s.db.QueryRowContext(ctx, query, opID.String()).Scan(
		&op.ID,
		&op.DocID,
		&op.UserID,
		&op.SessionID,
		&op.Type,
		&op.Data,
		&op.Timestamp,
		&op.Version,
		&op.PrevVersion,
		&op.Status,
		&op.ClientID,
		&op.Metadata.IP,
		&op.Metadata.UserAgent,
		&op.Metadata.Platform,
		&extra,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("operation not found: %s", opID.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get operation: %w", err)
	}

	if len(extra) > 0 {
		json.Unmarshal(extra, &op.Metadata.Extra)
	}

	return &op, nil
}

// UpdateOperation updates an existing operation
func (s *PostgresStore) UpdateOperation(ctx context.Context, op *Operation) error {
	query := `
		UPDATE operations SET
			status = $2,
			user_agent = $3,
			platform = $4,
			extra = $5
		WHERE id = $1`

	extra, err := json.Marshal(op.Metadata.Extra)
	if err != nil {
		return fmt.Errorf("failed to marshal extra metadata: %w", err)
	}

	result, err := s.db.ExecContext(ctx, query,
		op.ID.String(),
		string(op.Status),
		op.Metadata.UserAgent,
		op.Metadata.Platform,
		extra,
	)

	if err != nil {
		return fmt.Errorf("failed to update operation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("operation not found: %s", op.ID.String())
	}

	return nil
}

// ListOperations lists operations with filtering
func (s *PostgresStore) ListOperations(ctx context.Context, filter *OperationFilter) ([]*Operation, int, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		FROM operations
		WHERE 1=1`
	
	countQuery := `SELECT COUNT(*) FROM operations WHERE 1=1`
	
	args := []interface{}{}
	argIndex := 1

	// Apply filters
	if filter != nil {
		if filter.DocID != nil {
			query += fmt.Sprintf(" AND doc_id = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND doc_id = $%d", argIndex)
			args = append(args, filter.DocID.String())
			argIndex++
		}
		if filter.UserID != nil {
			query += fmt.Sprintf(" AND user_id = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND user_id = $%d", argIndex)
			args = append(args, *filter.UserID)
			argIndex++
		}
		if filter.Status != nil {
			query += fmt.Sprintf(" AND status = $%d", argIndex)
			countQuery += fmt.Sprintf(" AND status = $%d", argIndex)
			args = append(args, string(*filter.Status))
			argIndex++
		}
	}

	query += " ORDER BY timestamp DESC"
	
	if filter != nil {
		if filter.Limit > 0 {
			query += fmt.Sprintf(" LIMIT $%d", argIndex)
			args = append(args, filter.Limit)
			argIndex++
		}
		if filter.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, filter.Offset)
			argIndex++
		}
	}

	// Get total count
	var total int
	countArgs := args
	if filter != nil && (filter.Limit > 0 || filter.Offset > 0) {
		countArgs = args[:len(args)-2]
	}
	err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count operations: %w", err)
	}

	// Query operations
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list operations: %w", err)
	}
	defer rows.Close()

	operations := []*Operation{}
	for rows.Next() {
		var op Operation
		var extra []byte

		err := rows.Scan(
			&op.ID,
			&op.DocID,
			&op.UserID,
			&op.SessionID,
			&op.Type,
			&op.Data,
			&op.Timestamp,
			&op.Version,
			&op.PrevVersion,
			&op.Status,
			&op.ClientID,
			&op.Metadata.IP,
			&op.Metadata.UserAgent,
			&op.Metadata.Platform,
			&extra,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan operation: %w", err)
		}

		if len(extra) > 0 {
			json.Unmarshal(extra, &op.Metadata.Extra)
		}

		operations = append(operations, &op)
	}

	return operations, total, nil
}

// GetOperationsByDocument gets operations for a document
func (s *PostgresStore) GetOperationsByDocument(ctx context.Context, docID guuid.UUID, limit int) ([]*Operation, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		FROM operations
		WHERE doc_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, docID.String(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get operations by document: %w", err)
	}
	defer rows.Close()

	operations := []*Operation{}
	for rows.Next() {
		var op Operation
		var extra []byte

		err := rows.Scan(
			&op.ID,
			&op.DocID,
			&op.UserID,
			&op.SessionID,
			&op.Type,
			&op.Data,
			&op.Timestamp,
			&op.Version,
			&op.PrevVersion,
			&op.Status,
			&op.ClientID,
			&op.Metadata.IP,
			&op.Metadata.UserAgent,
			&op.Metadata.Platform,
			&extra,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		if len(extra) > 0 {
			json.Unmarshal(extra, &op.Metadata.Extra)
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

// GetOperationsByVersion gets operations in version range
func (s *PostgresStore) GetOperationsByVersion(ctx context.Context, docID guuid.UUID, minVersion, maxVersion uint64) ([]*Operation, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		FROM operations
		WHERE doc_id = $1 AND version >= $2 AND version <= $3
		ORDER BY version ASC`

	rows, err := s.db.QueryContext(ctx, query, docID.String(), minVersion, maxVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get operations by version: %w", err)
	}
	defer rows.Close()

	operations := []*Operation{}
	for rows.Next() {
		var op Operation
		var extra []byte

		err := rows.Scan(
			&op.ID,
			&op.DocID,
			&op.UserID,
			&op.SessionID,
			&op.Type,
			&op.Data,
			&op.Timestamp,
			&op.Version,
			&op.PrevVersion,
			&op.Status,
			&op.ClientID,
			&op.Metadata.IP,
			&op.Metadata.UserAgent,
			&op.Metadata.Platform,
			&extra,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		if len(extra) > 0 {
			json.Unmarshal(extra, &op.Metadata.Extra)
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

// GetPendingOperations gets pending operations
func (s *PostgresStore) GetPendingOperations(ctx context.Context, docID guuid.UUID) ([]*Operation, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, type, data,
			timestamp, version, prev_version, status, client_id,
			ip, user_agent, platform, extra
		FROM operations
		WHERE doc_id = $1 AND status = 'pending'
		ORDER BY timestamp ASC`

	rows, err := s.db.QueryContext(ctx, query, docID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get pending operations: %w", err)
	}
	defer rows.Close()

	operations := []*Operation{}
	for rows.Next() {
		var op Operation
		var extra []byte

		err := rows.Scan(
			&op.ID,
			&op.DocID,
			&op.UserID,
			&op.SessionID,
			&op.Type,
			&op.Data,
			&op.Timestamp,
			&op.Version,
			&op.PrevVersion,
			&op.Status,
			&op.ClientID,
			&op.Metadata.IP,
			&op.Metadata.UserAgent,
			&op.Metadata.Platform,
			&extra,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		if len(extra) > 0 {
			json.Unmarshal(extra, &op.Metadata.Extra)
		}

		operations = append(operations, &op)
	}

	return operations, nil
}

// ==================== 冲突管理 ====================

// CreateConflict creates a new conflict record
func (s *PostgresStore) CreateConflict(ctx context.Context, conflict *Conflict) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert conflict
	query := `
		INSERT INTO conflicts (
			id, doc_id, resolution, resolved_by, resolved_at, description
		) VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = tx.ExecContext(ctx, query,
		conflict.ID.String(),
		conflict.DocID.String(),
		string(conflict.Resolution),
		conflict.ResolvedBy,
		conflict.ResolvedAt,
		conflict.Description,
	)
	if err != nil {
		return fmt.Errorf("failed to create conflict: %w", err)
	}

	// Insert conflict operations
	if len(conflict.Ops) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
			INSERT INTO conflict_operations (conflict_id, operation_id) 
			VALUES ($1, $2)`)
		if err != nil {
			return fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, op := range conflict.Ops {
			_, err = stmt.ExecContext(ctx, conflict.ID.String(), op.ID.String())
			if err != nil {
				return fmt.Errorf("failed to insert conflict operation: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetConflict retrieves a conflict by ID
func (s *PostgresStore) GetConflict(ctx context.Context, conflictID guuid.UUID) (*Conflict, error) {
	query := `
		SELECT 
			id, doc_id, resolution, resolved_by, resolved_at, description
		FROM conflicts
		WHERE id = $1`

	var conflict Conflict
	var resolvedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, conflictID.String()).Scan(
		&conflict.ID,
		&conflict.DocID,
		&conflict.Resolution,
		&conflict.ResolvedBy,
		&resolvedAt,
		&conflict.Description,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("conflict not found: %s", conflictID.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get conflict: %w", err)
	}

	if resolvedAt.Valid {
		conflict.ResolvedAt = resolvedAt.Time
	}

	// Get conflict operations
	opsQuery := `
		SELECT o.id, o.doc_id, o.user_id, o.session_id, o.type, o.data,
			o.timestamp, o.version, o.prev_version, o.status, o.client_id,
			o.ip, o.user_agent, o.platform, o.extra
		FROM operations o
		JOIN conflict_operations co ON o.id = co.operation_id
		WHERE co.conflict_id = $1`

	rows, err := s.db.QueryContext(ctx, opsQuery, conflictID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get conflict operations: %w", err)
	}
	defer rows.Close()

	conflict.Ops = []*Operation{}
	for rows.Next() {
		var op Operation
		var extra []byte

		err := rows.Scan(
			&op.ID,
			&op.DocID,
			&op.UserID,
			&op.SessionID,
			&op.Type,
			&op.Data,
			&op.Timestamp,
			&op.Version,
			&op.PrevVersion,
			&op.Status,
			&op.ClientID,
			&op.Metadata.IP,
			&op.Metadata.UserAgent,
			&op.Metadata.Platform,
			&extra,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan operation: %w", err)
		}

		if len(extra) > 0 {
			json.Unmarshal(extra, &op.Metadata.Extra)
		}

		conflict.Ops = append(conflict.Ops, &op)
	}

	return &conflict, nil
}

// UpdateConflict updates an existing conflict
func (s *PostgresStore) UpdateConflict(ctx context.Context, conflict *Conflict) error {
	query := `
		UPDATE conflicts SET
			resolution = $2,
			resolved_by = $3,
			resolved_at = $4,
			description = $5
		WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query,
		conflict.ID.String(),
		string(conflict.Resolution),
		conflict.ResolvedBy,
		conflict.ResolvedAt,
		conflict.Description,
	)

	if err != nil {
		return fmt.Errorf("failed to update conflict: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("conflict not found: %s", conflict.ID.String())
	}

	return nil
}

// ListConflicts lists conflicts for a document
func (s *PostgresStore) ListConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error) {
	query := `
		SELECT 
			id, doc_id, resolution, resolved_by, resolved_at, description
		FROM conflicts
		WHERE doc_id = $1
		ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, docID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to list conflicts: %w", err)
	}
	defer rows.Close()

	conflicts := []*Conflict{}
	for rows.Next() {
		var conflict Conflict
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&conflict.ID,
			&conflict.DocID,
			&conflict.Resolution,
			&conflict.ResolvedBy,
			&resolvedAt,
			&conflict.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conflict: %w", err)
		}

		if resolvedAt.Valid {
			conflict.ResolvedAt = resolvedAt.Time
		}

		conflicts = append(conflicts, &conflict)
	}

	return conflicts, nil
}

// GetUnresolvedConflicts gets unresolved conflicts
func (s *PostgresStore) GetUnresolvedConflicts(ctx context.Context, docID guuid.UUID) ([]*Conflict, error) {
	query := `
		SELECT 
			id, doc_id, resolution, resolved_by, resolved_at, description
		FROM conflicts
		WHERE doc_id = $1 AND resolved_at IS NULL
		ORDER BY created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, docID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get unresolved conflicts: %w", err)
	}
	defer rows.Close()

	conflicts := []*Conflict{}
	for rows.Next() {
		var conflict Conflict

		err := rows.Scan(
			&conflict.ID,
			&conflict.DocID,
			&conflict.Resolution,
			&conflict.ResolvedBy,
			&conflict.ResolvedAt,
			&conflict.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conflict: %w", err)
		}

		conflicts = append(conflicts, &conflict)
	}

	return conflicts, nil
}

// ==================== 锁管理 ====================

// AcquireLock acquires a lock on a document
func (s *PostgresStore) AcquireLock(ctx context.Context, lock *Lock) error {
	query := `
		INSERT INTO locks (
			id, doc_id, user_id, session_id, acquired_at, expires_at, active
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := s.db.ExecContext(ctx, query,
		lock.ID.String(),
		lock.DocID.String(),
		lock.UserID,
		lock.SessionID.String(),
		lock.AcquiredAt,
		lock.ExpiresAt,
		lock.Active,
	)

	if err != nil {
		// Check for unique constraint violation
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" { // unique_violation
				return fmt.Errorf("document is already locked")
			}
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	return nil
}

// ReleaseLock releases a lock
func (s *PostgresStore) ReleaseLock(ctx context.Context, docID guuid.UUID, userID string) error {
	query := `
		UPDATE locks SET active = FALSE
		WHERE doc_id = $1 AND user_id = $2 AND active = TRUE`

	result, err := s.db.ExecContext(ctx, query, docID.String(), userID)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("no active lock found for document %s by user %s", docID.String(), userID)
	}

	return nil
}

// GetLock retrieves lock information
func (s *PostgresStore) GetLock(ctx context.Context, docID guuid.UUID) (*Lock, error) {
	query := `
		SELECT 
			id, doc_id, user_id, session_id, acquired_at, expires_at, active
		FROM locks
		WHERE doc_id = $1 AND active = TRUE
		ORDER BY acquired_at DESC
		LIMIT 1`

	var lock Lock

	err := s.db.QueryRowContext(ctx, query, docID.String()).Scan(
		&lock.ID,
		&lock.DocID,
		&lock.UserID,
		&lock.SessionID,
		&lock.AcquiredAt,
		&lock.ExpiresAt,
		&lock.Active,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No active lock
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get lock: %w", err)
	}

	return &lock, nil
}

// IsLocked checks if a document is locked
func (s *PostgresStore) IsLocked(ctx context.Context, docID guuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM locks 
			WHERE doc_id = $1 AND active = TRUE AND expires_at > CURRENT_TIMESTAMP
		)`

	var locked bool
	err := s.db.QueryRowContext(ctx, query, docID.String()).Scan(&locked)
	if err != nil {
		return false, fmt.Errorf("failed to check lock: %w", err)
	}

	return locked, nil
}

// CleanExpiredLocks cleans up expired locks
func (s *PostgresStore) CleanExpiredLocks(ctx context.Context) (int, error) {
	query := `SELECT clean_expired_locks()`

	var count int
	err := s.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to clean expired locks: %w", err)
	}

	return count, nil
}

// ==================== 统计信息 ====================

// GetStats retrieves statistics
func (s *PostgresStore) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		LastUpdated: time.Now(),
	}

	// Get document counts
	err := s.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN state = 'active' THEN 1 END) as active,
			COUNT(CASE WHEN state = 'archived' THEN 1 END) as archived
		FROM documents
	`).Scan(&stats.TotalDocuments, &stats.ActiveDocuments, &stats.ArchivedDocuments)
	if err != nil {
		return nil, fmt.Errorf("failed to get document stats: %w", err)
	}

	// Get operation count
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM operations`).Scan(&stats.TotalOperations)
	if err != nil {
		return nil, fmt.Errorf("failed to get operation stats: %w", err)
	}

	// Get conflict counts
	err = s.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN resolved_at IS NOT NULL THEN 1 END) as resolved
		FROM conflicts
	`).Scan(&stats.TotalConflicts, &stats.ResolvedConflicts)
	if err != nil {
		return nil, fmt.Errorf("failed to get conflict stats: %w", err)
	}

	// Get active locks count
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM locks WHERE active = TRUE AND expires_at > CURRENT_TIMESTAMP
	`).Scan(&stats.ActiveLocks)
	if err != nil {
		return nil, fmt.Errorf("failed to get lock stats: %w", err)
	}

	// Active subscribers is not tracked in DB (it's in-memory in Manager)
	stats.ActiveSubscribers = 0

	return stats, nil
}

// CountDocuments counts documents
func (s *PostgresStore) CountDocuments(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE state != 'deleted'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	return count, nil
}

// CountOperations counts operations
func (s *PostgresStore) CountOperations(ctx context.Context, docID *guuid.UUID) (int, error) {
	var count int
	var err error

	if docID != nil {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM operations WHERE doc_id = $1`, docID.String()).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM operations`).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to count operations: %w", err)
	}
	return count, nil
}

// CountConflicts counts conflicts
func (s *PostgresStore) CountConflicts(ctx context.Context, docID *guuid.UUID) (int, error) {
	var count int
	var err error

	if docID != nil {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conflicts WHERE doc_id = $1`, docID.String()).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM conflicts`).Scan(&count)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to count conflicts: %w", err)
	}
	return count, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// Ping checks database connectivity
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
