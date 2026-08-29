// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

// RedisConfig declares the configuration read by the Redis cache provider.
type RedisConfig struct {
	Enabled            bool     `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs              []string `config:"addrs" env:"REDIS_ADDR"`
	Username           string   `config:"username" env:"REDIS_USERNAME"`
	Password           string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB                 int      `config:"db" env:"REDIS_DB"`
	ClusterMode        bool     `config:"cluster_mode" env:"REDIS_CLUSTER_MODE"`
	MasterName         string   `config:"master_name" env:"REDIS_MASTER_NAME"`
	KeyPrefix          string   `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	PoolSize           int      `config:"pool_size" env:"REDIS_POOL_SIZE"`
	MinIdleConn        int      `config:"min_idle_conn" env:"REDIS_MIN_IDLE_CONN"`
	DialTimeout        int      `config:"dial_timeout" env:"REDIS_DIAL_TIMEOUT"`
	ReadTimeout        int      `config:"read_timeout" env:"REDIS_READ_TIMEOUT"`
	WriteTimeout       int      `config:"write_timeout" env:"REDIS_WRITE_TIMEOUT"`
	MaxRetries         int      `config:"max_retries" env:"REDIS_MAX_RETRIES"`
	PoolTimeout        int      `config:"pool_timeout" env:"REDIS_POOL_TIMEOUT"`
	ConnMaxIdleTime    int      `config:"conn_max_idle_time" env:"REDIS_CONN_MAX_IDLE_TIME"`
	MaintNotifications bool     `config:"maint_notifications" env:"REDIS_MAINT_NOTIFICATIONS" default:"false"`
}
