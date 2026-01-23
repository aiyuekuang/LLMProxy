package discovery

import (
	"context"
	"database/sql"
	"fmt"

	"llmproxy/internal/config"
)

// DatabaseSource 数据库发现源
// 从数据库表中读取后端服务列表
type DatabaseSource struct {
	BaseSource
	db        *sql.DB
	driver    string
	tableName string
	fields    map[string]string
}

// NewDatabaseSource 创建数据库发现源
// 参数：
//   - name: 发现源名称
//   - db: 数据库连接
//   - driver: 数据库驱动类型
//   - cfg: 数据库发现配置
//
// 返回：
//   - Source: 发现源实例
//   - error: 错误信息
func NewDatabaseSource(name string, db *sql.DB, driver string, cfg *config.DiscoveryDatabaseConfig) (Source, error) {
	if db == nil {
		return nil, fmt.Errorf("数据库连接为空")
	}

	tableName := cfg.Table
	if tableName == "" {
		tableName = "services"
	}

	// 默认字段映射
	fields := map[string]string{
		"name":   "name",
		"url":    "url",
		"weight": "weight",
		"status": "status",
	}

	// 合并自定义字段映射
	if cfg.Fields != nil {
		for k, v := range cfg.Fields {
			fields[k] = v
		}
	}

	return &DatabaseSource{
		BaseSource: NewBaseSource(name, "database"),
		db:         db,
		driver:     driver,
		tableName:  tableName,
		fields:     fields,
	}, nil
}

// Discover 从数据库读取后端服务列表
func (d *DatabaseSource) Discover(ctx context.Context) ([]*config.Backend, error) {
	// 构建查询语句
	query := fmt.Sprintf(
		"SELECT %s, %s, %s FROM %s WHERE %s = 'enabled' OR %s = 'active'",
		d.fields["name"], d.fields["url"], d.fields["weight"],
		d.tableName,
		d.fields["status"], d.fields["status"],
	)

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询数据库失败: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var backends []*config.Backend
	for rows.Next() {
		var name, url string
		var weight int
		if err := rows.Scan(&name, &url, &weight); err != nil {
			continue // 跳过错误行
		}

		if weight <= 0 {
			weight = 1
		}

		backends = append(backends, &config.Backend{
			Name:   name,
			URL:    url,
			Weight: weight,
		})
	}

	return backends, nil
}
