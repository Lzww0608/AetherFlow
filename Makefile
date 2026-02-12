.PHONY: all proto build-gateway build-session build-statesync build clean run-gateway run-session run-statesync test docker

# 变量定义
PROTO_DIR := api/proto
OUT_DIR := api/proto
GO := go
PROTOC := protoc

# 编译所有
all: proto build

# 生成 proto 文件
proto:
	@echo "Generating protobuf files..."
	@$(PROTOC) --go_out=$(OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/session.proto
	@$(PROTOC) --go_out=$(OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(OUT_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/statesync.proto
	@echo "Proto files generated successfully"

# 编译所有服务
build: build-gateway build-session build-statesync

# 编译 Gateway
build-gateway:
	@echo "Building Gateway..."
	@$(GO) build -o bin/gateway cmd/gateway/main.go
	@echo "Gateway built successfully"

# 编译 Session Service
build-session:
	@echo "Building Session Service..."
	@$(GO) build -o bin/session-service cmd/session-service/main.go
	@echo "Session Service built successfully"

# 编译 StateSync Service
build-statesync:
	@echo "Building StateSync Service..."
	@$(GO) build -o bin/statesync-service cmd/statesync-service/main.go
	@echo "StateSync Service built successfully"

# 清理编译产物
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@echo "Clean complete"

# 运行 Gateway
run-gateway:
	@echo "Running Gateway..."
	@./bin/gateway -f configs/gateway.yaml

# 运行 Session Service
run-session:
	@echo "Running Session Service..."
	@./bin/session-service -f configs/session.yaml

# 运行 StateSync Service
run-statesync:
	@echo "Running StateSync Service..."
	@./bin/statesync-service -f configs/statesync.yaml

# 测试
test:
	@echo "Running tests..."
	@$(GO) test -v ./...

# Docker 构建
docker:
	@echo "Building Docker images..."
	@docker build -t aetherflow/gateway:latest -f deployments/Dockerfile.gateway .
	@docker build -t aetherflow/session-service:latest -f deployments/Dockerfile.session .
	@docker build -t aetherflow/statesync-service:latest -f deployments/Dockerfile.statesync .
	@echo "Docker images built successfully"

# 帮助信息
help:
	@echo "AetherFlow Makefile Commands:"
	@echo "  make proto            - Generate protobuf files"
	@echo "  make build            - Build all services"
	@echo "  make build-gateway    - Build Gateway only"
	@echo "  make build-session    - Build Session Service only"
	@echo "  make build-statesync  - Build StateSync Service only"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make run-gateway      - Run Gateway"
	@echo "  make run-session      - Run Session Service"
	@echo "  make run-statesync    - Run StateSync Service"
	@echo "  make test             - Run all tests"
	@echo "  make docker           - Build Docker images"
	@echo "  make help             - Show this help message"
