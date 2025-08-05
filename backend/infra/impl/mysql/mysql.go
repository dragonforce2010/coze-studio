/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mysql

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func New() (*gorm.DB, error) {
	dsn := os.Getenv("MYSQL_DSN")
	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		return nil, fmt.Errorf("mysql open, dsn: %s, err: %w", dsn, err)
	}

	// 获取底层的 sql.DB 实例
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB failed: %w", err)
	}

	// 设置连接池参数
	// 最大开放连接数 - 根据并发需求调整
	maxOpenConns := getEnvInt("MYSQL_MAX_OPEN_CONNS", 100)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	// 最大空闲连接数 - 通常设置为 MaxOpenConns 的 1/2 到 1/4
	maxIdleConns := getEnvInt("MYSQL_MAX_IDLE_CONNS", 25)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	// 连接最大生命周期 - 防止长时间连接导致的问题
	connMaxLifetime := getEnvDuration("MYSQL_CONN_MAX_LIFETIME", 5*time.Minute)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// 连接最大空闲时间
	connMaxIdleTime := getEnvDuration("MYSQL_CONN_MAX_IDLE_TIME", 10*time.Minute)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)

	return db, nil
}

// getEnvInt 获取环境变量中的整数值，如果不存在则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration 获取环境变量中的时间间隔值，如果不存在则返回默认值
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
