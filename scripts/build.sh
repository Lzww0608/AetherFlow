#!/bin/bash

# AetherFlow Build Script
# This script builds all services and creates Docker images

set -euo pipefail

# Configuration
PROJECT_NAME="aetherflow"
REGISTRY="${DOCKER_REGISTRY:-localhost:5000}"
VERSION="${VERSION:-$(git describe --tags --always --dirty)}"
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Services to build
SERVICES=("api-gateway" "session-service" "statesync-service")

# Functions
log() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
    fi
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed or not in PATH"
    fi
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]]; then
        error "go.mod not found. Please run this script from the project root."
    fi
    
    success "Prerequisites check passed"
}

# Generate protobuf files
generate_proto() {
    log "Generating protobuf files..."
    
    if ! command -v protoc &> /dev/null; then
        warn "protoc not found, skipping protobuf generation"
        return 0
    fi
    
    # Create output directory
    mkdir -p api/proto/gen
    
    # Generate Go files from proto definitions
    find api/proto -name "*.proto" -exec protoc \
        --go_out=api/proto/gen \
        --go_opt=paths=source_relative \
        --go-grpc_out=api/proto/gen \
        --go-grpc_opt=paths=source_relative \
        {} \;
    
    success "Protobuf files generated"
}

# Run tests
run_tests() {
    log "Running tests..."
    
    # Unit tests
    go test -v -race -coverprofile=coverage.out ./...
    
    # Check coverage
    COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    log "Test coverage: ${COVERAGE}%"
    
    if (( $(echo "$COVERAGE < 80" | bc -l) )); then
        warn "Test coverage is below 80%"
    fi
    
    success "Tests passed"
}

# Build binary for a specific service
build_service() {
    local service=$1
    log "Building ${service}..."
    
    # Create bin directory
    mkdir -p bin
    
    # Build the service
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
        -a -installsuffix cgo \
        -o "bin/${service}" \
        "./cmd/${service}"
    
    success "Built ${service}"
}

# Build Docker image for a specific service
build_docker_image() {
    local service=$1
    local image_name="${REGISTRY}/${PROJECT_NAME}/${service}:${VERSION}"
    local latest_name="${REGISTRY}/${PROJECT_NAME}/${service}:latest"
    
    log "Building Docker image for ${service}..."
    
    # Build the image
    docker build \
        --build-arg SERVICE_NAME="${service}" \
        --build-arg VERSION="${VERSION}" \
        --build-arg BUILD_TIME="${BUILD_TIME}" \
        --build-arg GIT_COMMIT="${GIT_COMMIT}" \
        -t "${image_name}" \
        -t "${latest_name}" \
        .
    
    success "Built Docker image: ${image_name}"
}

# Push Docker images
push_images() {
    if [[ "${PUSH_IMAGES:-false}" == "true" ]]; then
        log "Pushing Docker images..."
        
        for service in "${SERVICES[@]}"; do
            local image_name="${REGISTRY}/${PROJECT_NAME}/${service}:${VERSION}"
            local latest_name="${REGISTRY}/${PROJECT_NAME}/${service}:latest"
            
            log "Pushing ${image_name}..."
            docker push "${image_name}"
            docker push "${latest_name}"
        done
        
        success "Images pushed to registry"
    else
        log "Skipping image push (set PUSH_IMAGES=true to enable)"
    fi
}

# Clean build artifacts
clean() {
    log "Cleaning build artifacts..."
    rm -rf bin/
    rm -rf api/proto/gen/
    rm -f coverage.out
    success "Cleaned build artifacts"
}

# Main build process
main() {
    log "Starting AetherFlow build process..."
    log "Version: ${VERSION}"
    log "Build Time: ${BUILD_TIME}"
    log "Git Commit: ${GIT_COMMIT}"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --clean)
                clean
                exit 0
                ;;
            --no-tests)
                SKIP_TESTS=true
                shift
                ;;
            --no-docker)
                SKIP_DOCKER=true
                shift
                ;;
            --push)
                PUSH_IMAGES=true
                shift
                ;;
            --service)
                SINGLE_SERVICE="$2"
                shift 2
                ;;
            -h|--help)
                echo "Usage: $0 [OPTIONS]"
                echo "Options:"
                echo "  --clean       Clean build artifacts and exit"
                echo "  --no-tests    Skip running tests"
                echo "  --no-docker   Skip Docker image building"
                echo "  --push        Push Docker images to registry"
                echo "  --service     Build only specified service"
                echo "  -h, --help    Show this help message"
                exit 0
                ;;
            *)
                error "Unknown option: $1"
                ;;
        esac
    done
    
    # Check prerequisites
    check_prerequisites
    
    # Generate protobuf files
    generate_proto
    
    # Run tests unless skipped
    if [[ "${SKIP_TESTS:-false}" != "true" ]]; then
        run_tests
    fi
    
    # Determine which services to build
    if [[ -n "${SINGLE_SERVICE:-}" ]]; then
        if [[ " ${SERVICES[*]} " =~ " ${SINGLE_SERVICE} " ]]; then
            SERVICES=("${SINGLE_SERVICE}")
        else
            error "Unknown service: ${SINGLE_SERVICE}"
        fi
    fi
    
    # Build services
    for service in "${SERVICES[@]}"; do
        build_service "${service}"
        
        # Build Docker images unless skipped
        if [[ "${SKIP_DOCKER:-false}" != "true" ]]; then
            build_docker_image "${service}"
        fi
    done
    
    # Push images if requested
    push_images
    
    success "Build completed successfully!"
    log "Built services: ${SERVICES[*]}"
    log "Version: ${VERSION}"
}

# Run main function
main "$@"
