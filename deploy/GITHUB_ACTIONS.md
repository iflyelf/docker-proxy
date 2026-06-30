# GitHub Actions 配置指南

本项目已配置 GitHub Actions 自动构建多架构 Docker 镜像。

## 📋 配置 Secrets

在仓库中配置以下 Secrets 以启用自动推送到 Docker Hub：

1. 访问 https://github.com/iflyelf/docker-proxy/settings/secrets/actions
2. 点击 **New repository secret** 添加以下 2 个 secrets：

| Secret 名称 | 值 | 说明 |
|------------|-----|------|
| `DOCKER_USERNAME` | `iflyelf` | Docker Hub 用户名 |
| `DOCKER_PASSWORD` | `dckr_pat_xxxxx` | Docker Hub 访问令牌（非密码）|

### 获取 Docker Hub Token

1. 登录 https://hub.docker.com/settings/security
2. 点击 **New Access Token**
3. Token name: `github-actions-docker-proxy`
4. Permissions: **Read, Write, Delete**（或 **Read & Write**）
5. 复制生成的 token（仅显示一次）
6. 将 token 填入 `DOCKER_PASSWORD` secret

## 🚀 触发构建

配置完 secrets 后，以下操作会触发自动构建：

### 1. Push 代码

```bash
# 修改任何源码或 Dockerfile
git add -A
git commit -m "feat: 更新功能"
git push
```

### 2. 手动触发

1. 访问 https://github.com/iflyelf/docker-proxy/actions
2. 选择 **构建并发布 Docker 镜像** workflow
3. 点击 **Run workflow** → 选择分支 `main` → **Run workflow**

### 3. Star 仓库触发

点击仓库右上角 ⭐ Star 也会触发构建（首次 Star 时）

## 📦 构建产物

构建成功后，镜像会推送到：

- **Docker Hub**: `iflyelf/docker-proxy:latest`
- **支持架构**: `linux/amd64`, `linux/arm64`

验证：

```bash
docker pull iflyelf/docker-proxy:latest
docker run --rm iflyelf/docker-proxy:latest /app/docker-proxy --help
```

## 🔍 查看构建状态

- **Actions 页面**: https://github.com/iflyelf/docker-proxy/actions
- **Badge 徽章**（可添加到 README.md）:

```markdown
[![Docker Build](https://github.com/iflyelf/docker-proxy/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/iflyelf/docker-proxy/actions/workflows/docker-publish.yml)
```

## 📝 Workflow 说明

文件: `.github/workflows/docker-publish.yml`

### 关键步骤

1. **检出代码** (`actions/checkout@v4`)
2. **安装 QEMU** (`docker/setup-qemu-action@v3`) - 支持 ARM64 模拟
3. **安装 Buildx** (`docker/setup-buildx-action@v3`) - 多架构构建
4. **登录 Docker Hub** (`docker/login-action@v3`) - 使用 secrets
5. **构建并推送** (`docker/build-push-action@v6`) - 同时构建 amd64/arm64
6. **清理旧 runs** (`Mattraks/delete-workflow-runs@main`) - 节省存储空间

### 构建时间

- **首次构建**: ~5-8 分钟（Go 依赖下载 + 多架构编译）
- **后续构建**: ~2-4 分钟（利用 GitHub Actions cache）

## ⚠️ 常见问题

### 1. 构建失败：`Error: Cannot perform an interactive login from a non TTY device`

**原因**: `DOCKER_PASSWORD` secret 未配置或值错误

**解决**: 确认已正确添加 secret，值为 Docker Hub token（非密码）

### 2. 构建成功但无法 pull 镜像

**原因**: Docker Hub 镜像未公开

**解决**: 登录 https://hub.docker.com/repository/docker/iflyelf/docker-proxy/general → 确保仓库为 **Public**

### 3. 推送失败：`denied: requested access to the resource is denied`

**原因**: token 权限不足或已过期

**解决**: 重新生成 token（需要 Read & Write 权限），更新 `DOCKER_PASSWORD` secret

## 🎯 手动触发特定标签

如需发布特定版本（如 `v1.0.0`），修改 workflow：

```yaml
# .github/workflows/docker-publish.yml
matrix:
  include:
    - DOCKER_TAG: latest
    - DOCKER_TAG: v1.0.0  # 添加版本标签
```

或在 Git 中打 tag：

```bash
git tag v1.0.0
git push origin v1.0.0
```

然后修改 workflow 触发条件：

```yaml
on:
  push:
    tags:
      - 'v*'
```

## 📚 参考文档

- [GitHub Actions 文档](https://docs.github.com/actions)
- [Docker Buildx 多架构构建](https://docs.docker.com/build/building/multi-platform/)
- [Docker Hub 访问令牌](https://docs.docker.com/docker-hub/access-tokens/)
