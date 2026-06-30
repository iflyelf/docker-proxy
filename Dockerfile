##########################################
#         构建可执行二进制文件             #
##########################################
# 指定创建的基础镜像
FROM alpine:latest as builder

# 作者描述信息
LABEL org.opencontainers.image.authors="iflyelf" \
      org.opencontainers.image.vendor="iflyelf"

# buildx 自动注入的目标架构 (amd64/arm64/arm/386), 用于按架构编译对应二进制
ARG TARGETARCH
ARG TARGETVARIANT

# 时区设置
ARG TZ=Asia/Shanghai
ENV TZ=$TZ
# 语言设置
ARG LANG=C.UTF-8
ENV LANG=$LANG

# GO环境变量
ARG GO_VERSION=1.26.4
ENV GO_VERSION=$GO_VERSION
ARG GOROOT=/opt/go
ENV GOROOT=$GOROOT
ARG GOPATH=/opt/golang
ENV GOPATH=$GOPATH
# Go 模块代理(加速依赖下载, 国内构建必备; 海外可改为 https://proxy.golang.org,direct)
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY
ENV PATH=$GOROOT/bin:$GOPATH/bin:$PATH

# 构建依赖
ARG BUILD_DEPS="\
      git \
      wget \
      curl \
      tar \
      make \
      gcc \
      musl-dev \
      ca-certificates"
ENV BUILD_DEPS=$BUILD_DEPS

# ***** 安装依赖 *****
RUN set -eux && \
   # 修改源地址
   sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
   # 更新源地址并更新系统软件
   apk update && apk upgrade && \
   # 安装依赖包
   apk add --no-cache --clean-protected $BUILD_DEPS && \
   rm -rf /var/cache/apk/* && \
   # 更新时区
   ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
   # 更新时间
   echo ${TZ} > /etc/timezone

# ***** 安装 Go *****
RUN set -eux && \
    case "${TARGETARCH}" in \
        amd64) GO_ARCH="amd64" ;; \
        arm64) GO_ARCH="arm64" ;; \
        arm)   GO_ARCH="armv6l" ;; \
        386)   GO_ARCH="386" ;; \
        *) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;; \
    esac && \
    wget --no-check-certificate -O /tmp/go.tar.gz \
        "https://golang.google.cn/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz" && \
    mkdir -p ${GOROOT} && \
    tar -C ${GOROOT} --strip-components=1 -xzf /tmp/go.tar.gz && \
    rm -f /tmp/go.tar.gz && \
    go version

# 工作目录
WORKDIR /build

# 复制源码
COPY go.mod ./
COPY *.go ./

# ***** 编译可执行二进制文件 *****
RUN set -eux && \
    # 下载依赖
    go mod download || true && \
    # 交叉编译(静态链接)
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /tmp/docker-proxy .



# ##############################################################################

##########################################
#         构建基础镜像                    #
##########################################
#
# 指定创建的基础镜像
FROM ubuntu:resolute

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
ARG LANG=zh_CN.UTF-8
ENV LANG=$LANG

# 环境设置
ARG DEBIAN_FRONTEND=noninteractive
ENV DEBIAN_FRONTEND=$DEBIAN_FRONTEND

# 镜像变量
ARG DOCKER_IMAGE=iflyelf/docker-proxy
ENV DOCKER_IMAGE=$DOCKER_IMAGE
ARG DOCKER_IMAGE_OS=ubuntu
ENV DOCKER_IMAGE_OS=$DOCKER_IMAGE_OS
ARG DOCKER_IMAGE_TAG=resolute
ENV DOCKER_IMAGE_TAG=$DOCKER_IMAGE_TAG

# 工作目录
ARG WORK_DIR=/app
ENV WORK_DIR=$WORK_DIR

# 安装依赖包
ARG PKG_DEPS="\
    bash \
    bash-completion \
    bind9-dnsutils \
    iproute2 \
    net-tools \
    ncat \
    vim \
    jq \
    tzdata \
    curl \
    wget \
    lsof \
    iputils-ping \
    telnet \
    procps \
    ca-certificates \
    locales"
ENV PKG_DEPS=$PKG_DEPS

# 拷贝 docker-proxy 二进制
COPY --from=builder /tmp/docker-proxy /usr/local/bin/docker-proxy

# 拷贝配置和文档
COPY ["./deploy", "/app/deploy"]
COPY ["./config.example.json", "/app/config.example.json"]
COPY ["./README.md", "/app/README.md"]

# ***** 安装依赖 *****
RUN set -eux && \
   # 更新源地址
   sed -i 's@URIs: http://[a-z.]*\.ubuntu\.com/ubuntu/@URIs: https://mirrors.aliyun.com/ubuntu/@g' /etc/apt/sources.list.d/ubuntu.sources && \
   sed -i 's@^Types: deb$@Types: deb deb-src@' /etc/apt/sources.list.d/ubuntu.sources && \
   # 解决证书认证失败问题
   touch /etc/apt/apt.conf.d/99verify-peer.conf && echo >>/etc/apt/apt.conf.d/99verify-peer.conf "Acquire { https::Verify-Peer false }" && \
   # 更新系统软件
   DEBIAN_FRONTEND=noninteractive apt-get update -qqy && apt-get upgrade -qqy && \
   # 安装依赖包
   DEBIAN_FRONTEND=noninteractive apt-get install -qqy --no-install-recommends $PKG_DEPS --option=Dpkg::Options::=--force-confdef && \
   DEBIAN_FRONTEND=noninteractive apt-get -qqy --no-install-recommends autoremove --purge && \
   DEBIAN_FRONTEND=noninteractive apt-get -qqy --no-install-recommends autoclean && \
   rm -rf /var/lib/apt/lists/* && \
   # 更新时区
   ln -sf /usr/share/zoneinfo/${TZ} /etc/localtime && \
   # 更新时间
   echo ${TZ} > /etc/timezone && \
   # 配置中文环境
   locale-gen zh_CN.UTF-8 && localedef -f UTF-8 -i zh_CN zh_CN.UTF-8 && locale-gen

# ***** 检查依赖并授权 *****
RUN set -eux && \
    # 创建用户和用户组
    addgroup --system --quiet docker-proxy && \
    adduser --quiet --system --disabled-login --ingroup docker-proxy --home ${WORK_DIR} --no-create-home docker-proxy && \
    # 授权
    chmod a+x /usr/local/bin/docker-proxy && \
    # smoke test
    # ##############################################################################
    docker-proxy --help || true && \
    rm -rf /var/lib/apt/lists/* /tmp/*

# ***** 工作目录 *****
WORKDIR ${WORK_DIR}

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
    CMD curl --fail http://localhost:5000/health || exit 1
