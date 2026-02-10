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
	
	// JWT配置
	JWT JWTConfig `json:",optional"`
	
	// gRPC配置
	GRPC GRPCConfig `json:",optional"`
	
	// 链路追踪配置
	Tracing TracingConfig `json:",optional"`
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
	Enable      bool     `json:",default=false"`                      // 是否启用Etcd
	Endpoints   []string `json:",default=[127.0.0.1:2379]"`           // Etcd endpoints
	DialTimeout int      `json:",default=5"`                          // 连接超时（秒）
	Username    string   `json:",optional"`                           // 用户名
	Password    string   `json:",optional"`                           // 密码
	ServiceTTL  int64    `json:",default=10"`                         // 服务注册TTL（秒）
	ServiceName string   `json:",default=aetherflow-gateway"`         // 服务名称
	ServiceAddr string   `json:",default=localhost:8888"`             // 服务地址
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
	Enable           bool   `json:",default=true"`               // 是否启用熔断
	Threshold        float64 `json:",default=0.5"`                // 错误率阈值
	MinRequests      uint32  `json:",default=5"`                  // 最小请求数
	ConsecutiveFailures uint32 `json:",default=5"`               // 连续失败阈值
	Timeout          int     `json:",default=60"`                 // 熔断超时（秒）
	HalfOpenRequests uint32  `json:",default=3"`                  // 半开状态最大请求数
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	Enable   bool `json:",default=true"`
	Interval int  `json:",default=10"` // 秒
	Timeout  int  `json:",default=3"`  // 秒
}

// JWTConfig JWT配置
type JWTConfig struct {
	Secret        string `json:",default=aetherflow-secret-key"` // JWT密钥
	Expire        int64  `json:",default=86400"`                  // 过期时间（秒，默认24小时）
	RefreshExpire int64  `json:",default=604800"`                 // 刷新令牌过期时间（秒，默认7天）
	Issuer        string `json:",default=aetherflow"`             // 签发者
}

// GRPCConfig gRPC配置
type GRPCConfig struct {
	Session      ServiceEndpoint    `json:",optional"`
	StateSync    ServiceEndpoint    `json:",optional"`
	Pool         PoolConfig         `json:",optional"`
	LoadBalancer LoadBalancerConfig `json:",optional"`
}

// ServiceEndpoint 服务端点配置
type ServiceEndpoint struct {
	Target         string `json:",default=127.0.0.1:9001"`              // 服务地址（静态）
	Timeout        int    `json:",default=5000"`                         // 超时时间（毫秒）
	MaxRetries     int    `json:",default=3"`                            // 最大重试次数
	Transport      string `json:",default=tcp,options=tcp|quantum"`      // 传输协议 (tcp/quantum)
	UseDiscovery   bool   `json:",default=false"`                        // 是否使用服务发现
	DiscoveryName  string `json:",optional"`                             // 服务发现名称
}

// PoolConfig 连接池配置
type PoolConfig struct {
	MaxIdle     int `json:",default=10"`  // 最大空闲连接数
	MaxActive   int `json:",default=100"` // 最大活跃连接数
	IdleTimeout int `json:",default=60"`  // 空闲超时（秒）
}

// LoadBalancerConfig 负载均衡配置
type LoadBalancerConfig struct {
	Policy string `json:",default=round_robin"` // 负载均衡策略
}

// TracingConfig 链路追踪配置
type TracingConfig struct {
	Enable       bool    `json:",default=false"`                              // 是否启用追踪
	ServiceName  string  `json:",default=aetherflow-gateway"`                 // 服务名称
	Endpoint     string  `json:",default=http://localhost:14268/api/traces"`  // Jaeger endpoint
	Exporter     string  `json:",default=jaeger,options=jaeger|zipkin"`       // 导出器类型
	SampleRate   float64 `json:",default=1.0"`                                // 采样率 (0.0-1.0)
	Environment  string  `json:",default=development"`                         // 环境
	BatchTimeout int     `json:",default=5"`                                  // 批量发送超时（秒）
	MaxQueueSize int     `json:",default=2048"`                               // 最大队列大小
}
