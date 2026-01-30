package config

import "github.com/zeromicro/go-zero/rest"

// Config API Gateway配置
type Config struct {
	rest.RestConf

	// 日志配置
	Log LogConfig `json:",optional"`

	// CORS配置
	Cors CorsConfig `json:",optional"`

	// Etcd配置
	Etcd EtcdConfig `json:",optional"`

	// 服务配置
	SessionService  ServiceConfig `json:",optional"`
	StateSyncService ServiceConfig `json:",optional"`

	// 限流配置
	RateLimit RateLimitConfig `json:",optional"`

	// 熔断配置
	Breaker BreakerConfig `json:",optional"`

	// 健康检查配置
	Health HealthConfig `json:",optional"`
}

// LogConfig 日志配置
type LogConfig struct {
	ServiceName          string `json:",default=aetherflow-gateway"`
	Mode                 string `json:",default=console,options=console|file|volume"`
	Path                 string `json:",default=logs/gateway"`
	Level                string `json:",default=info,options=debug|info|warn|error"`
	Compress             bool   `json:",default=false"`
	KeepDays             int    `json:",default=7"`
	StackCooldownMillis  int    `json:",default=100"`
}

// CorsConfig CORS配置
type CorsConfig struct {
	Enable           bool     `json:",default=true"`
	AllowOrigins     []string `json:",optional"`
	AllowMethods     []string `json:",optional"`
	AllowHeaders     []string `json:",optional"`
	ExposeHeaders    []string `json:",optional"`
	AllowCredentials bool     `json:",default=true"`
	MaxAge           int      `json:",default=3600"`
}

// EtcdConfig Etcd配置
type EtcdConfig struct {
	Hosts []string `json:",optional"`
	Key   string   `json:",default=aetherflow/services"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Endpoints []string `json:",optional"`
	Timeout   int64    `json:",default=5000"` // 毫秒
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enable bool `json:",default=true"`
	Rate   int  `json:",default=100"` // 每秒请求数
	Burst  int  `json:",default=200"` // 突发容量
}

// BreakerConfig 熔断配置
type BreakerConfig struct {
	Enable    bool    `json:",default=true"`
	Threshold float64 `json:",default=0.5"` // 错误率阈值
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Enable   bool `json:",default=true"`
	Interval int  `json:",default=10"` // 秒
	Timeout  int  `json:",default=3"`  // 秒
}
