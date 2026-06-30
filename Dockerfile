# 多阶段构建：编译 + 运行
# 支持 linux/amd64 和 linux/arm64

#===========================================
# 阶段 1: 构建
#===========================================
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine AS builder

# 构建参数（buildx 自动注入）
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# 工作目录
WORKDIR /build

# 复制 go.mod（利用 Docker 缓存）
COPY go.mod ./

# 下载依赖（如有）
RUN go mod download || true

# 复制源码
COPY *.go ./

# 交叉编译（静态链接）
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o docker-proxy .

#===========================================
# 阶段 2: 运行
#===========================================
FROM alpine:latest

# 元数据
LABEL org.opencontainers.image.authors="iflyelf" \
      org.opencontainers.image.vendor="iflyelf" \
      org.opencontainers.image.title="docker-proxy" \
      org.opencontainers.image.description="Docker Registry Proxy with multi-registry support" \
      org.opencontainers.image.source="https://github.com/iflyelf/docker-proxy"

# 安装 CA 证书（HTTPS 访问上游 registry 必需）
RUN apk --no-cache add ca-certificates tzdata

# 时区设置（默认上海）
ENV TZ=Asia/Shanghai

# 工作目录
WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /build/docker-proxy /app/docker-proxy

# 复制部署配置（可选）
COPY deploy/ /app/deploy/
COPY config.example.json /app/config.example.json
COPY README.md /app/README.md

# 暴露端口（根据默认配置）
# docker.io=5000, quay.io=5001, gcr.io=5002, k8s.gcr.io=5003
# registry.k8s.io=5004, ghcr.io=5005, nvcr.io=5008
EXPOSE 5000 5001 5002 5003 5004 5005 5008

# 启动代理（使用内置默认配置）
CMD ["/app/docker-proxy"]
