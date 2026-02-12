package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aetherflow/aetherflow/cmd/session-service/config"
	"github.com/aetherflow/aetherflow/cmd/session-service/server"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	configFile = flag.String("f", "configs/session.yaml", "配置文件路径")
	version    = "0.1.0"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// 创建 logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}
	defer logger.Sync()

	logger.Info("Starting Session Service",
		zap.String("version", version),
		zap.String("build_time", buildTime))

	// 加载配置
	cfg, err := loadConfig(*configFile)
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// 创建服务器
	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create server", zap.Error(err))
	}

	// 启动服务器（在新的 goroutine 中）
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errCh <- err
		}
	}()

	// 等待中断信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		logger.Fatal("Server error", zap.Error(err))
	case sig := <-sigCh:
		logger.Info("Received signal", zap.String("signal", sig.String()))
		srv.Stop()
	}

	logger.Info("Session Service shutdown complete")
}

// loadConfig 加载配置文件
func loadConfig(filename string) (*config.Config, error) {
	// 读取文件
	data, err := os.ReadFile(filename)
	if err != nil {
		// 如果文件不存在，使用默认配置
		if os.IsNotExist(err) {
			fmt.Printf("Config file not found, using default config\n")
			return config.DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析 YAML
	cfg := config.DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}
