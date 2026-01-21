package pipeline

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql" // MySQL 驱动
	_ "github.com/lib/pq"              // PostgreSQL 驱动
	_ "modernc.org/sqlite"             // SQLite 驱动（纯 Go，无需 CGO）
)

// DatabaseProvider 数据库 Provider
// 从 MySQL/PostgreSQL/SQLite 读取 API Key 信息
type DatabaseProvider struct {
	BaseProvider
	db        *sql.DB  // 数据库连接
	table     string   // 表名
	keyColumn string   // API Key 列名
	fields    []string // 需要查询的字段
}

// NewDatabaseProvider 创建数据库 Provider
// 参数：
//   - name: Provider 名称
//   - cfg: 数据库配置
// 返回：
//   - Provider: Provider 实例
//   - error: 错误信息
func NewDatabaseProvider(name string, cfg *DatabaseConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("数据库配置不能为空")
	}

	// 兼容旧配置：如果直接配置了连接信息
	if cfg.Storage == "" {
		// 验证配置
		if cfg.Driver == "" {
			return nil, fmt.Errorf("数据库驱动不能为空")
		}
		if cfg.DSN == "" {
			return nil, fmt.Errorf("数据库 DSN 不能为空")
		}
		if cfg.Table == "" {
			return nil, fmt.Errorf("表名不能为空")
		}
		if cfg.KeyColumn == "" {
			cfg.KeyColumn = "api_key"
		}

		// 打开数据库连接
		db, err := sql.Open(cfg.Driver, cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("数据库连接失败: %w", err)
		}

		// 测试连接
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("数据库 Ping 失败: %w", err)
		}

		// 设置连接池
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)

		return &DatabaseProvider{
			BaseProvider: BaseProvider{
				name:         name,
				providerType: ProviderTypeDatabase,
			},
			db:        db,
			table:     cfg.Table,
			keyColumn: cfg.KeyColumn,
			fields:    cfg.Fields,
		}, nil
	}

	// 新配置：需要传入已创建的连接
	return nil, fmt.Errorf("Database Provider 需要使用 NewDatabaseProviderWithDB 创建")
}

// NewDatabaseProviderWithDB 使用已创建的数据库连接创建 Provider
func NewDatabaseProviderWithDB(name string, db interface{}, cfg *DatabaseConfig) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("数据库配置不能为空")
	}

	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return nil, fmt.Errorf("无效的数据库连接类型")
	}

	if cfg.Table == "" {
		return nil, fmt.Errorf("表名不能为空")
	}

	keyColumn := cfg.KeyColumn
	if keyColumn == "" {
		keyColumn = "api_key"
	}

	return &DatabaseProvider{
		BaseProvider: BaseProvider{
			name:         name,
			providerType: ProviderTypeDatabase,
		},
		db:        sqlDB,
		table:     cfg.Table,
		keyColumn: keyColumn,
		fields:    cfg.Fields,
	}, nil
}

// Query 查询 API Key 信息
// 参数：
//   - ctx: 上下文
//   - apiKey: API Key 字符串
// 返回：
//   - *ProviderResult: 查询结果
func (d *DatabaseProvider) Query(ctx context.Context, apiKey string) *ProviderResult {
	// 构建查询字段
	selectFields := "*"
	if len(d.fields) > 0 {
		selectFields = strings.Join(d.fields, ", ")
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", selectFields, d.table, d.keyColumn)

	// 执行查询
	rows, err := d.db.QueryContext(ctx, query, apiKey)
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("数据库查询失败: %w", err),
		}
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("获取列信息失败: %w", err),
		}
	}

	// 读取数据
	if !rows.Next() {
		return &ProviderResult{Found: false}
	}

	// 创建接收器
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// 扫描数据
	if err := rows.Scan(valuePtrs...); err != nil {
		return &ProviderResult{
			Found: false,
			Error: fmt.Errorf("数据扫描失败: %w", err),
		}
	}

	// 转换为 map
	data := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		// 处理 []byte 类型
		if b, ok := val.([]byte); ok {
			data[col] = string(b)
		} else {
			data[col] = val
		}
	}

	return &ProviderResult{
		Found: true,
		Data:  data,
	}
}

// Close 关闭数据库连接
func (d *DatabaseProvider) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}
