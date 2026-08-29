// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

type queueConfig struct {
	Name     string `config:"name"`
	Priority int    `config:"priority"`
}

type workerConfig struct {
	Concurrency    int           `config:"concurrency" env:"WORKER_CONCURRENCY" default:"10"`
	StrictPriority bool          `config:"strict_priority" env:"WORKER_STRICT_PRIORITY" default:"false"`
	Queues         []queueConfig `config:"queues"`
}

type redisWorkerConfig struct {
	Enabled            bool     `config:"enabled" env:"REDIS_ENABLED" default:"false" autoEnable:"REDIS_ADDR"`
	Addrs              []string `config:"addrs" env:"REDIS_ADDR"`
	Username           string   `config:"username" env:"REDIS_USERNAME"`
	Password           string   `config:"password" env:"REDIS_PASSWORD" secret:"true"`
	DB                 int      `config:"db" env:"REDIS_DB"`
	ClusterMode        bool     `config:"cluster_mode" env:"REDIS_CLUSTER_MODE"`
	MasterName         string   `config:"master_name" env:"REDIS_MASTER_NAME"`
	KeyPrefix          string   `config:"key_prefix" env:"REDIS_KEY_PREFIX"`
	PoolSize           int      `config:"pool_size" env:"REDIS_POOL_SIZE"`
	MaintNotifications bool     `config:"maint_notifications" env:"REDIS_MAINT_NOTIFICATIONS" default:"false"`
}
