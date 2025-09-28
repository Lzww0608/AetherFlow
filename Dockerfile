# Multi-stage Dockerfile for AetherFlow services
# Stage 1: Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG SERVICE_NAME
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.gitCommit=${GIT_COMMIT}" \
    -a -installsuffix cgo \
    -o /app/bin/service \
    ./cmd/${SERVICE_NAME}

# Stage 2: Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 aetherflow && \
    adduser -D -s /bin/sh -u 1000 -G aetherflow aetherflow

# Set timezone
ENV TZ=UTC

# Copy binary from builder stage
COPY --from=builder /app/bin/service /usr/local/bin/aetherflow-service

# Copy configuration files
COPY --from=builder /app/configs /etc/aetherflow/

# Set ownership
RUN chown -R aetherflow:aetherflow /etc/aetherflow/

# Switch to non-root user
USER aetherflow

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /usr/local/bin/aetherflow-service --health-check || exit 1

# Expose ports (will be overridden by specific services)
EXPOSE 8080 9090

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/aetherflow-service"]

# Default command
CMD ["--help"]

# Labels
LABEL maintainer="AetherFlow Team <team@aetherflow.io>"
LABEL org.opencontainers.image.title="AetherFlow Service"
LABEL org.opencontainers.image.description="High-performance real-time collaboration service with Quantum protocol"
LABEL org.opencontainers.image.vendor="AetherFlow"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/aetherflow/aetherflow"
LABEL org.opencontainers.image.documentation="https://docs.aetherflow.io"
