package config

import "time"

// Config StateSync Service 配置
type Config struct {
	Server  ServerConfig  `yaml:"Server"`
	Store   StoreConfig   `yaml:"Store"`
	Log     LogConfig     `yaml:"Log"`
	Metrics MetricsConfig `yaml:"Metrics"`
	Tracing TracingConfig `yaml:"Tracing"`
	Manager ManagerConfig `yaml:"Manager"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `yaml:"Host"`
	Port int    `yaml:"Port"`
}

// StoreConfig 存储配置
type StoreConfig struct {
	Type       string            `yaml:"Type"` // memory, postgres
	Postgres   PostgresConfig    `yaml:"Postgres,omitempty"`
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host         string `yaml:"Host"`
	Port         int    `yaml:"Port"`
	User         string `yaml:"User"`
	Password     string `yaml:"Password"`
	DBName       string `yaml:"DBName"`
	SSLMode      string `yaml:"SSLMode"`
	MaxOpenConns int    `yaml:"MaxOpenConns"`
	MaxIdleConns int    `yaml:"MaxIdleConns"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `yaml:"Level"`  // debug, info, warn, error
	Format string `yaml:"Format"` // json, console
}

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enable bool   `yaml:"Enable"`
	Host   string `yaml:"Host"`
	Port   int    `yaml:"Port"`
	Path   string `yaml:"Path"`
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enable       bool    `yaml:"Enable"`
	ServiceName  string  `yaml:"ServiceName"`
	Endpoint     string  `yaml:"Endpoint"`
	Exporter     string  `yaml:"Exporter"`
	SampleRate   float64 `yaml:"SampleRate"`
	Environment  string  `yaml:"Environment"`
	BatchTimeout int     `yaml:"BatchTimeout"`
	MaxQueueSize int     `yaml:"MaxQueueSize"`
}

// ManagerConfig Manager 配置
type ManagerConfig struct {
	LockTimeout          time.Duration `yaml:"LockTimeout"`
	CleanupInterval      time.Duration `yaml:"CleanupInterval"`
	AutoResolveConflicts bool          `yaml:"AutoResolveConflicts"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9002,
		},
		Store: StoreConfig{
			Type: "memory",
			Postgres: PostgresConfig{
				Host:         "localhost",
				Port:         5432,
				User:         "postgres",
				Password:     "postgres",
				DBName:       "aetherflow",
				SSLMode:      "disable",
				MaxOpenConns: 25,
				MaxIdleConns: 5,
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enable: true,
			Host:   "0.0.0.0",
			Port:   9102,
			Path:   "/metrics",
		},
		Tracing: TracingConfig{
			Enable:       false,
			ServiceName:  "statesync-service",
			Endpoint:     "http://localhost:14268/api/traces",
			Exporter:     "jaeger",
			SampleRate:   1.0,
			Environment:  "development",
			BatchTimeout: 5,
			MaxQueueSize: 2048,
		},
		Manager: ManagerConfig{
			LockTimeout:          30 * time.Second,
			CleanupInterval:      5 * time.Minute,
			AutoResolveConflicts: true,
		},
	}
}
