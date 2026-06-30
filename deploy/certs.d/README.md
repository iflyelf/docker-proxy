# containerd / k8s 镜像加速配置说明

containerd 通过 `/etc/containerd/certs.d/<registry>/hosts.toml` 配置镜像源。
将本目录下的文件按对应路径放置即可。

目录结构：

```
/etc/containerd/certs.d/
├── docker.io/hosts.toml          -> https://docker.xiaonuo.live
├── gcr.io/hosts.toml             -> https://gcr.iflyelf.com
├── k8s.gcr.io/hosts.toml         -> https://k8s-gcr.iflyelf.com
├── registry.k8s.io/hosts.toml    -> https://k8s.iflyelf.com
├── quay.io/hosts.toml            -> https://quay.iflyelf.com
├── ghcr.io/hosts.toml            -> https://ghcr.nju.edu.cn
└── nvcr.io/hosts.toml            -> https://ngc.nju.edu.cn
```

确保 `/etc/containerd/config.toml` 中启用了 certs.d：

```toml
[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
```

修改后重启 containerd：`systemctl restart containerd`
