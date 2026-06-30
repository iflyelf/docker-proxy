#!/bin/bash
# 交叉编译 Linux x86-64

set -e

echo "🛠️  交叉编译 Linux x86-64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o docker-proxy-linux-amd64 .

echo "✅ 编译完成: docker-proxy-linux-amd64"
ls -lh docker-proxy-linux-amd64
