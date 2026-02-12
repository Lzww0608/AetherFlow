package config

import "time"

// Config Session Service 配置
type Config struct {
	Server  ServerConfig  `yaml:"Server"`
	Store   StoreConfig   `yaml:"Store"`
	Log     LogConfig     `yaml:"Log"`
	Metrics MetricsConfig `yaml:"Metrics"`
	Tracing TracingConfig `yaml:"Tracing"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `yaml:"Host"`
	Port int    `yaml:"Port"`
}

// StoreConfig 存储配置
type StoreConfig struct {
	Type  string      `yaml:"Type"` // memory, redis
	Redis RedisConfig `yaml:"Redis,omitempty"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr         string        `yaml:"Addr"`
	Password     string        `yaml:"Password"`
	DB           int           `yaml:"DB"`
	PoolSize     int           `yaml:"PoolSize"`
	MinIdleConns int           `yaml:"MinIdleConns"`
	MaxRetries   int           `yaml:"MaxRetries"`
	DialTimeout  time.Duration `yaml:"DialTimeout"`
	ReadTimeout  time.Duration `yaml:"ReadTimeout"`
	WriteTimeout time.Duration `yaml:"WriteTimeout"`
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

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 9001,
		},
		Store: StoreConfig{
			Type: "memory",
			Redis: RedisConfig{
				Addr:         "localhost:6379",
				PoolSize:     10,
				MinIdleConns: 5,
				MaxRetries:   3,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enable: true,
			Host:   "0.0.0.0",
			Port:   9101,
			Path:   "/metrics",
		},
		Tracing: TracingConfig{
			Enable:       false,
			ServiceName:  "session-service",
			Endpoint:     "http://localhost:14268/api/traces",
			Exporter:     "jaeger",
			SampleRate:   1.0,
			Environment:  "development",
			BatchTimeout: 5,
			MaxQueueSize: 2048,
		},
	}
}
