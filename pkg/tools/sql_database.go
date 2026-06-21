package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// SQLDatabaseTool 简单的 SQL 查询工具
// 直接执行 SQL 查询（不包含自然语言转换）
type SQLDatabaseTool struct {
	name   string
	desc   string
	db     *sql.DB
	logger *zap.Logger
}

// Name returns the tool name.
func (t *SQLDatabaseTool) Name() string { return t.name }

// Description returns the tool description.
func (t *SQLDatabaseTool) Description() string { return t.desc }

// NewSQLDatabaseTool 创建 SQL 数据库工具
func NewSQLDatabaseTool(db *sql.DB, logger *zap.Logger) *SQLDatabaseTool {
	logger.Info("SQL Database Tool initialized")

	return &SQLDatabaseTool{
		name: "sql_query",
		desc: `执行 SQL 查询并返回结果。
功能：直接执行 SQL SELECT 查询并返回 JSON 格式的结果。
输入格式：SQL 查询语句（字符串）
返回：查询结果（JSON 数组）

示例输入: "SELECT COUNT(*) as count FROM users"
返回: [{"count": 100}]

注意：仅支持 SELECT 查询，不支持 INSERT/UPDATE/DELETE 等修改操作。`,
		db:     db,
		logger: logger,
	}
}

// Call 执行工具
func (t *SQLDatabaseTool) Call(ctx context.Context, input string) (string, error) {
	query := strings.TrimSpace(input)
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	// 安全检查：只允许 SELECT 查询
	queryLower := strings.ToLower(query)
	if !strings.HasPrefix(queryLower, "select") {
		return "", fmt.Errorf("only SELECT queries are allowed")
	}

	t.logger.Info("Executing SQL query", zap.String("query", query))

	// 执行查询
	rows, err := t.db.QueryContext(ctx, query)
	if err != nil {
		t.logger.Error("SQL query failed", zap.Error(err))
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	// 读取结果
	results := []map[string]interface{}{}
	for rows.Next() {
		// 创建值容器
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// 扫描行
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		// 构建结果映射
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("rows error: %w", err)
	}

	// 序列化结果
	resultJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	t.logger.Info("SQL query completed",
		zap.Int("rows", len(results)))

	return string(resultJSON), nil
}

// Close 关闭数据库连接
func (t *SQLDatabaseTool) Close() error {
	if t.db != nil {
		return t.db.Close()
	}
	return nil
}
