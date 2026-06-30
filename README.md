# Docker Registry Proxy

[![Docker Build](https://github.com/iflyelf/docker-proxy/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/iflyelf/docker-proxy/actions/workflows/docker-publish.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/iflyelf/docker-proxy)](https://hub.docker.com/r/iflyelf/docker-proxy)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

自建 Docker 镜像代理服务，支持 docker.io、gcr.io、k8s.gcr.io、registry.k8s.io、quay.io、ghcr.io、nvcr.io 等多个上游 registry，部署在可访问上游的海外服务器上，解决国内无法直接拉取 Docker 镜像的问题。

## ✨ 功能特性

- **多 registry 支持**：每个 registry 独立端口监听，配置灵活
- **透明 token 鉴权**：自动处理 WWW-Authenticate 质询，无需客户端配置认证
- **Docker Hub 增强**：
  - 自动补全官方镜像 `library/` 前缀
  - Token 缓存避免重复请求 auth.docker.io
  - 浏览器访问显示镜像搜索页面（服务端代理 Docker Hub 搜索，支持电脑/手机端 H5）
- **Blob 下载优化**：CDN 重定向自动跟随，支持多跳
- **私有镜像支持**：透传 `docker login` 凭证（Basic Auth / Bearer Token）
- **健康检查**：`/health` 端点诊断到上游的连通性
- **生产就绪**：
  - 支持 TLS (HTTPS)
  - 支持后台守护进程模式
  - systemd 集成
  - 单文件编译，无外部依赖

## 🏗️ 架构

```
Docker Client (国内)
    │
    │  docker pull nginx
    │  docker pull quay.io/prometheus/node-exporter
    │
    ▼
Nginx (443/80)
    │
    ├─→ docker-cf.iflyelf.com  → :5000 (docker.io)
    ├─→ quay-cf.iflyelf.com    → :5001 (quay.io)
    ├─→ gcr-cf.iflyelf.com     → :5002 (gcr.io)
    ├─→ k8s-gcr-cf.iflyelf.com → :5003 (k8s.gcr.io)
    ├─→ k8s-cf.iflyelf.com     → :5004 (registry.k8s.io)
    ├─→ ghcr-cf.iflyelf.com    → :5005 (ghcr.io)
    └─→ nvcr-cf.iflyelf.com    → :5008 (nvcr.io)
    │
    ▼
Docker Proxy (Go)
    │
    ├─→ auth.docker.io / auth.quay.io   (获取 token，带缓存)
    ├─→ registry-1.docker.io / quay.io  (拉取 manifest)
    └─→ CDN / S3                        (跟随重定向，下载 blob)
    │
    ▼
Docker Client 收到镜像数据
```

## 📦 编译

需要 Go 1.26+。

### 本机编译

```bash
go build -o docker-proxy .
```

### 交叉编译 Linux x86-64

```bash
bash build.sh
# 或手动：
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o docker-proxy-linux-amd64 .
```

### Docker 镜像

**多架构镜像（推荐）**：

```bash
# 拉取预构建镜像（支持 linux/amd64, linux/arm64）
docker pull iflyelf/docker-proxy:latest

# 或本地构建多架构镜像
docker buildx build --platform linux/amd64,linux/arm64 -t iflyelf/docker-proxy:latest .
```

**单架构快速构建**：

```bash
docker build -t docker-proxy:latest .
```

## 🚀 部署

### 方式 A：Docker 部署（推荐）

#### 使用 docker run

```bash
docker run -d \
  --name docker-proxy \
  --restart unless-stopped \
  -p 5000:5000 \
  -p 5001:5001 \
  -p 5002:5002 \
  -p 5003:5003 \
  -p 5004:5004 \
  -p 5005:5005 \
  -p 5008:5008 \
  -e TZ=Asia/Shanghai \
  iflyelf/docker-proxy:latest
```

#### 使用 docker-compose

```bash
# 下载 docker-compose.yml
curl -O https://raw.githubusercontent.com/iflyelf/docker-proxy/main/docker-compose.yml

# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

#### 自定义配置

```bash
# 创建配置文件
cat > config.json << 'EOF'
{
  "default_port": 5000,
  "registries": [
    {
      "prefix": "docker.io",
      "port": 5000,
      "mirrors": [
        {
          "host": "https://your-mirror.example.com",
          "capabilities": ["pull", "resolve"]
        }
      ]
    }
  ]
}
EOF

# 挂载配置文件运行
docker run -d \
  --name docker-proxy \
  -p 5000-5008:5000-5008 \
  -v $(pwd)/config.json:/app/config.json:ro \
  iflyelf/docker-proxy:latest \
  /app/docker-proxy -config /app/config.json
```

### 方式 B：二进制部署

#### 1. 上传二进制

```bash
scp docker-proxy-linux-amd64 root@your-server:/opt/docker-proxy/docker-proxy
ssh root@your-server
chmod +x /opt/docker-proxy/docker-proxy
```

#### 2. 运行方式

**前台运行（调试用）**

```bash
cd /opt/docker-proxy
./docker-proxy
# 默认使用内置配置，监听端口 5000-5008
```

**后台守护进程**

```bash
./docker-proxy -d
# 日志输出到 docker-proxy.log
# PID 写入 docker-proxy.pid

# 停止
kill $(cat docker-proxy.pid)
```

**systemd 管理（推荐）**

```bash
# 复制 service 文件
cp deploy/docker-proxy.service /etc/systemd/system/

# 启动
sudo systemctl daemon-reload
sudo systemctl enable --now docker-proxy

# 查看状态
sudo systemctl status docker-proxy

# 查看日志
sudo journalctl -u docker-proxy -f
```

#### 3. 配置 Nginx 反向代理

```bash
# 复制配置
cp deploy/registry-proxy.conf /etc/nginx/conf.d/

# 修改 SSL 证书路径（如果使用 HTTPS）
# 编辑 registry-proxy.conf，调整 include /ssl/iflyelf.com/iflyelf.com.conf

# 测试配置
nginx -t

# 重启 Nginx
systemctl restart nginx
```

#### 4. DNS 配置

为以下域名添加 A 记录指向你的服务器 IP：

- `docker-cf.iflyelf.com`
- `quay-cf.iflyelf.com`
- `gcr-cf.iflyelf.com`
- `k8s-gcr-cf.iflyelf.com`
- `k8s-cf.iflyelf.com`
- `ghcr-cf.iflyelf.com`
- `nvcr-cf.iflyelf.com`

## 🔧 配置

### 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-config` | (空) | 配置文件路径 (JSON)，留空使用内置默认配置 |
| `-addr` | (空) | 单端口监听地址 (覆盖配置，仅代理 docker.io) |
| `-d` | `false` | 后台守护进程模式 |
| `-log` | `docker-proxy.log` | 日志文件路径（守护进程模式下生效） |

### 配置文件示例

默认配置已内置，如需自定义可创建 `config.json`：

```bash
cp config.example.json config.json
# 编辑 config.json
./docker-proxy -config config.json
```

配置格式见 `config.example.json`。

## 🐳 Docker 客户端配置

### 方式一：配置 registry-mirrors（推荐）

编辑 `/etc/docker/daemon.json`：

```json
{
  "registry-mirrors": ["https://docker-cf.iflyelf.com"],
  "insecure-registries": []
}
```

重启 Docker：

```bash
sudo systemctl daemon-reload
sudo systemctl restart docker
```

验证：

```bash
docker info | grep -A 5 "Registry Mirrors"
```

之后正常使用 `docker pull` 即可自动走代理：

```bash
docker pull nginx
docker pull ubuntu:22.04
docker pull quay.io/prometheus/node-exporter
```

> **注意**：如果使用 HTTPS（配置了 TLS 证书），则不需要 `insecure-registries`。

### 方式二：containerd / k8s 配置

containerd 通过 `/etc/containerd/certs.d/<registry>/hosts.toml` 配置镜像源。

```bash
# 复制配置文件
sudo cp -r deploy/certs.d/* /etc/containerd/certs.d/

# 确保 /etc/containerd/config.toml 中启用了 certs.d
# [plugins."io.containerd.grpc.v1.cri".registry]
#   config_path = "/etc/containerd/certs.d"

# 重启 containerd
sudo systemctl restart containerd
```

测试：

```bash
crictl pull docker.io/library/nginx:latest
crictl pull gcr.io/distroless/static-debian11:latest
```

## 🔍 镜像搜索

浏览器访问代理服务（如 `https://docker-cf.iflyelf.com`）即可使用镜像搜索功能：

1. **首页**：显示搜索框，输入关键词（如 nginx、redis）后提交
2. **搜索结果页**：
   - 响应式 H5 设计，支持电脑和手机端
   - 显示镜像名称、描述、stars、下载量
   - 一键复制 `docker pull` 命令
   - 服务端代理 Docker Hub 搜索 API（避免客户端直连被墙）

**工作原理**：
- 用户浏览器访问 `https://docker-cf.iflyelf.com/search?q=nginx`
- 代理服务（海外 VPS）请求 `https://hub.docker.com/v2/search/repositories/` API
- 将结果渲染为 HTML 返回给用户（无需客户端访问 Docker Hub）

**注意**：搜索功能要求代理服务部署在可访问 `hub.docker.com` 的海外服务器上。

## 📊 诊断

### 健康检查

访问 `/health` 端点检测代理到上游的连通性：

```bash
# docker.io (端口 5000)
curl http://localhost:5000/health

# quay.io (端口 5001)
curl http://localhost:5001/health
```

返回示例：

```json
{
  "registry": "docker.io",
  "mirror": "https://docker.xiaonuo.live",
  "status": "OK (HTTP 200)",
  "latency": "3.356s"
}
```

### V2 端点测试

```bash
curl http://localhost:5000/v2/
# 期望: {} （HTTP 200，带 Docker-Distribution-Api-Version 头）
```

### 手动测试镜像拉取

```bash
curl -s http://localhost:5000/v2/library/alpine/manifests/latest \
  -H "Accept: application/vnd.docker.distribution.manifest.v2+json" | head -20
```

### 查看日志

```bash
# systemd
sudo journalctl -u docker-proxy -f

# 守护进程模式
tail -f docker-proxy.log
```

## 🔐 私有镜像支持

代理透传 `docker login` 凭证，支持拉取私有镜像：

```bash
# 1. 先登录
docker login docker-cf.iflyelf.com
# 输入 Docker Hub 的用户名和密码

# 2. 拉取私有镜像
docker pull docker-cf.iflyelf.com/yourusername/private-image:tag
```

代理会自动：
1. 收到客户端发来的 Authorization 头（Basic Auth）
2. 向 auth.docker.io 请求 token 时透传该凭证
3. 用获取的 token 访问 registry-1.docker.io

## 🌐 支持的上游 registry

| Registry | 端口 | 示例镜像 |
|----------|------|---------|
| docker.io | 5000 | `nginx`, `mysql`, `redis` |
| quay.io | 5001 | `quay.io/prometheus/node-exporter` |
| gcr.io | 5002 | `gcr.io/distroless/static-debian11` |
| k8s.gcr.io | 5003 | `k8s.gcr.io/pause:3.9` |
| registry.k8s.io | 5004 | `registry.k8s.io/kube-apiserver:v1.28.0` |
| ghcr.io | 5005 | `ghcr.io/actions/runner:latest` |
| nvcr.io | 5008 | `nvcr.io/nvidia/cuda:12.0-base` |

## 🛠️ 故障排查

### 1. 端口被占用

```bash
# 检查端口占用
ss -tlnp | grep :5000

# 修改配置文件中的端口
```

### 2. 上游连接失败

```bash
# 测试服务器能否访问上游
curl -I https://docker.xiaonuo.live/v2/
curl -I https://registry-1.docker.io/v2/
```

### 3. Token 获取失败

查看日志中是否有 `获取 token 失败` 错误，检查：
- 上游 auth 服务是否可达（如 auth.docker.io）
- 私有镜像是否需要登录凭证

### 4. Docker 客户端报错 "unauthorized"

确认：
- 私有镜像需要先 `docker login`
- registry-mirrors 配置正确
- 镜像名称格式正确（官方镜像无需 `library/` 前缀）

## 📝 许可证

MIT

## 🙏 致谢

参考项目：
- [xiaoshouchen/docker-proxy](https://github.com/xiaoshouchen/docker-proxy) - 原始设计思路

---

**项目结构**

```
docker-proxy/
├── main.go              # 入口、多端口监听、守护进程
├── config.go            # 配置加载（JSON / 内置默认）
├── proxy.go             # 代理核心（V2 API、CDN 重定向）
├── token.go             # Token 鉴权（WWW-Authenticate 解析、缓存）
├── pages.go             # 搜索页 / nginx 伪装页
├── go.mod
├── build.sh             # 交叉编译脚本
├── config.example.json  # 配置示例
├── deploy/
│   ├── docker-proxy.service       # systemd unit
│   ├── registry-proxy.conf        # nginx 反向代理配置
│   └── certs.d/                   # containerd hosts.toml 配置
│       ├── docker.io/hosts.toml
│       ├── quay.io/hosts.toml
│       ├── gcr.io/hosts.toml
│       ├── k8s.gcr.io/hosts.toml
│       ├── registry.k8s.io/hosts.toml
│       ├── ghcr.io/hosts.toml
│       └── nvcr.io/hosts.toml
└── README.md            # 本文档
```
