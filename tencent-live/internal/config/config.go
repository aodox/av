package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Tencent     TencentConfig     `yaml:"tencent"`
	MySQL       MySQLConfig       `yaml:"mysql"`
	Redis       RedisConfig       `yaml:"redis"`
	AsyncWriter AsyncWriterConfig `yaml:"async_writer"`
	Monitor     MonitorConfig     `yaml:"monitor"`
	Log         LogConfig         `yaml:"log"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"`
}

type TencentConfig struct {
	SecretID      string `yaml:"secret_id"`
	SecretKey     string `yaml:"secret_key"`
	PushDomain    string `yaml:"push_domain"`
	PlayDomain    string `yaml:"play_domain"`
	AppName       string `yaml:"app_name"`
	PushAuthKey   string `yaml:"push_auth_key"`
	PlayAuthKey   string `yaml:"play_auth_key"`
	CallbackKey   string `yaml:"callback_key"` // 回调签名Key
	ExpireSeconds int64  `yaml:"expire_seconds"`
	Bizid         int64  `yaml:"bizid"`
	Region        string `yaml:"region"`
}

type MySQLConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	Database        string `yaml:"database"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // 连接最大生命周期(秒)
}

type RedisConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Password        string `yaml:"password"`
	DB              int    `yaml:"db"`
	PoolSize        int    `yaml:"pool_size"`
	MinIdleConns    int    `yaml:"min_idle_conns"`
	MaxRetries      int    `yaml:"max_retries"`
	DialTimeout     int    `yaml:"dial_timeout"`     // 连接超时(秒)
	ReadTimeout     int    `yaml:"read_timeout"`     // 读超时(秒)
	WriteTimeout    int    `yaml:"write_timeout"`    // 写超时(秒)
}

type AsyncWriterConfig struct {
	BatchSize  int `yaml:"batch_size"`   // 批量写入大小
	IntervalMs int `yaml:"interval_ms"`  // 写入间隔(毫秒)
}

type MonitorConfig struct {
	Enabled          bool `yaml:"enabled"`
	IntervalSeconds  int  `yaml:"interval_seconds"`
	MaxInactiveRetry int  `yaml:"max_inactive_retry"`
	MaxQPS           int  `yaml:"max_qps"`
	WorkerCount      int  `yaml:"worker_count"`
	BatchSize        int  `yaml:"batch_size"`
}

type LogConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	data = []byte(os.ExpandEnv(string(data)))

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	setDefaults(cfg)

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	if cfg.Tencent.ExpireSeconds == 0 {
		cfg.Tencent.ExpireSeconds = 86400
	}
	if cfg.Tencent.Region == "" {
		cfg.Tencent.Region = "ap-shanghai"
	}
	if cfg.Monitor.IntervalSeconds == 0 {
		cfg.Monitor.IntervalSeconds = 60
	}
	if cfg.Monitor.MaxInactiveRetry == 0 {
		cfg.Monitor.MaxInactiveRetry = 3
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.MaxSize == 0 {
		cfg.Log.MaxSize = 100
	}
	if cfg.Log.MaxBackups == 0 {
		cfg.Log.MaxBackups = 10
	}
	if cfg.Log.MaxAge == 0 {
		cfg.Log.MaxAge = 30
	}
	// Redis 高并发配置
	if cfg.Redis.PoolSize == 0 {
		cfg.Redis.PoolSize = 1000 // 高并发场景
	}
	if cfg.Redis.MinIdleConns == 0 {
		cfg.Redis.MinIdleConns = 100
	}
	if cfg.Redis.MaxRetries == 0 {
		cfg.Redis.MaxRetries = 3
	}
	if cfg.Redis.DialTimeout == 0 {
		cfg.Redis.DialTimeout = 5
	}
	if cfg.Redis.ReadTimeout == 0 {
		cfg.Redis.ReadTimeout = 3
	}
	if cfg.Redis.WriteTimeout == 0 {
		cfg.Redis.WriteTimeout = 3
	}
	// MySQL 高并发配置
	if cfg.MySQL.MaxOpenConns == 0 {
		cfg.MySQL.MaxOpenConns = 500 // 高并发场景
	}
	if cfg.MySQL.MaxIdleConns == 0 {
		cfg.MySQL.MaxIdleConns = 100
	}
	if cfg.MySQL.ConnMaxLifetime == 0 {
		cfg.MySQL.ConnMaxLifetime = 3600
	}
	// 异步写入配置
	if cfg.AsyncWriter.BatchSize == 0 {
		cfg.AsyncWriter.BatchSize = 1000 // 批量1000条
	}
	if cfg.AsyncWriter.IntervalMs == 0 {
		cfg.AsyncWriter.IntervalMs = 1000 // 1秒刷新一次
	}
}
