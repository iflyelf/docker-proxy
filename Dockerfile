##########################################
#         构建可执行二进制文件             #
##########################################
# 指定构建的基础镜像
FROM golang:1.26.4-alpine AS builder

# 作者描述信息
LABEL org.opencontainers.image.authors="iflyelf" \
      org.opencontainers.image.vendor="iflyelf"

# buildx 自动注入的目标架构 (amd64/arm64/arm/386)
ARG TARGETARCH
ARG TARGETVARIANT

# 时区设置
ARG TZ=Asia/Shanghai
ENV TZ=$TZ
# 语言设置
ARG LANG=C.UTF-8
ENV LANG=$LANG

# 构建依赖
ARG BUILD_DEPS="\
      git \
      wget \
      curl \
      make \
      gcc \
      musl-dev"
ENV BUILD_DEPS=$BUILD_DEPS

# ***** 安装依赖 *****
RUN set -eux && \
   # 修改源地址
   sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
   # 更新源地址并更新系统软件
   apk update && apk upgrade && \
   # 安装依赖包
   apk add --no-cache --clean-protected $BUILD_DEPS ca-certificates tzdata && \
   rm -rf /var/cache/apk/* && \
   # 更新时区
   ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
   # 更新时间
   echo ${TZ} > /etc/timezone

# 工作目录
WORKDIR /build

# 复制源码
COPY go.mod ./
COPY *.go ./

# Go 模块代理(加速依赖下载, 国内构建必备)
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY

# ***** 编译可执行二进制文件 *****
RUN set -eux && \
    # 下载依赖
    go mod download || true && \
    # 交叉编译(静态链接)
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o docker-proxy .



# ##############################################################################

##########################################
#         构建基础镜像                    #
##########################################
#
# 指定创建的基础镜像
FROM alpine:latest

# 作者描述信息
LABEL org.opencontainers.image.authors="iflyelf" \
      org.opencontainers.image.vendor="iflyelf"

# buildx 自动注入的目标架构 (amd64/arm64/arm/386)
ARG TARGETARCH
ARG TARGETVARIANT

# 时区设置
ARG TZ=Asia/Shanghai
ENV TZ=$TZ
# 语言设置
ARG LANG=C.UTF-8
ENV LANG=$LANG

# 镜像变量
ARG DOCKER_IMAGE=iflyelf/docker-proxy
ENV DOCKER_IMAGE=$DOCKER_IMAGE
ARG DOCKER_IMAGE_OS=alpine
ENV DOCKER_IMAGE_OS=$DOCKER_IMAGE_OS
ARG DOCKER_IMAGE_TAG=latest
ENV DOCKER_IMAGE_TAG=$DOCKER_IMAGE_TAG

# 安装依赖包
ARG PKG_DEPS="\
    bash \
    bash-completion \
    tzdata \
    curl \
    wget \
    ca-certificates \
    bind-tools \
    iputils \
    net-tools \
    iproute2 \
    procps \
    shadow"
ENV PKG_DEPS=$PKG_DEPS

# 拷贝二进制文件
COPY --from=builder /build/docker-proxy /usr/local/bin/docker-proxy

# ***** 安装依赖 *****
RUN set -eux && \
   # 修改源地址
   sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
   # 更新源地址并更新系统软件
   apk update && apk upgrade && \
   # 安装依赖包
   apk add --no-cache --clean-protected $PKG_DEPS && \
   rm -rf /var/cache/apk/* && \
   # 更新时区
   ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
   # 更新时间
   echo ${TZ} > /etc/timezone

# ***** 检查依赖并授权 *****
RUN set -eux && \
    # 创建用户和用户组
    addgroup -g 1000 -S docker-proxy && \
    adduser -S -D -H -u 1000 -h /app -s /sbin/nologin -G docker-proxy -g docker-proxy docker-proxy && \
    # 授权
    chmod a+x /usr/local/bin/docker-proxy && \
    # smoke test
    # ##############################################################################
    docker-proxy --help || true

# 复制部署配置（可选）
COPY deploy/ /app/deploy/
COPY config.example.json /app/config.example.json
COPY README.md /app/README.md

# ***** 工作目录 *****
WORKDIR /app

# 暴露端口（根据默认配置）
# docker.io=5000, quay.io=5001, gcr.io=5002, k8s.gcr.io=5003
# registry.k8s.io=5004, ghcr.io=5005, nvcr.io=5008
EXPOSE 5000 5001 5002 5003 5004 5005 5008

# ***** 容器信号处理 *****
STOPSIGNAL SIGTERM

# ***** 入口 *****
CMD ["/usr/local/bin/docker-proxy"]

# 自动检测服务是否可用
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:5000/health || exit 1
