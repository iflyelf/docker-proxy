package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	configPath string
	daemonize  bool
	logFile    string
	addrFlag   string
)

func main() {
	flag.StringVar(&configPath, "config", "", "配置文件路径 (JSON)，留空使用内置默认配置")
	flag.StringVar(&addrFlag, "addr", "", "单端口监听地址 (覆盖配置，仅代理 docker.io)")
	flag.BoolVar(&daemonize, "d", false, "后台守护进程模式")
	flag.StringVar(&logFile, "log", "docker-proxy.log", "日志文件路径（守护进程模式下生效）")
	flag.Parse()

	if daemonize {
		runDaemon()
		return
	}
	if os.Getenv("_DOCKER_PROXY_CHILD") == "1" {
		setupLogging()
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 单端口模式：仅代理 docker.io
	if addrFlag != "" {
		var dockerReg *RegistryConfig
		for i := range cfg.Registries {
			if cfg.Registries[i].Prefix == "docker.io" {
				dockerReg = &cfg.Registries[i]
				break
			}
		}
		if dockerReg == nil {
			log.Fatalf("配置中未找到 docker.io registry")
		}
		startSingleServer(addrFlag, cfg, dockerReg)
		return
	}

	startMultiServer(cfg)
}

// startSingleServer 启动单端口服务
func startSingleServer(addr string, cfg *Config, reg *RegistryConfig) {
	handler := NewProxyHandler(reg)
	server := newServer(addr, handler)

	go handleSignals(func() { server.Close() })

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		log.Printf("Docker 代理已启动 (HTTPS) %s -> %s", addr, reg.Prefix)
		if err := server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != http.ErrServerClosed {
			log.Fatalf("服务异常: %v", err)
		}
	} else {
		log.Printf("Docker 代理已启动 (HTTP) %s -> %s", addr, reg.Prefix)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("服务异常: %v", err)
		}
	}
}

// startMultiServer 按端口为每个 registry 启动独立监听
func startMultiServer(cfg *Config) {
	// 按端口分组（一个端口对应一个 registry）
	portMap := make(map[int]*RegistryConfig)
	for i := range cfg.Registries {
		reg := &cfg.Registries[i]
		port := reg.Port
		if port == 0 {
			port = cfg.DefaultPort
		}
		if _, exists := portMap[port]; exists {
			log.Printf("⚠️ 端口 %d 已被占用 (registry=%s)，跳过 %s", port, portMap[port].Prefix, reg.Prefix)
			continue
		}
		portMap[port] = reg
	}

	if len(portMap) == 0 {
		log.Fatalf("无可用 registry 配置")
	}

	var servers []*http.Server
	var wg sync.WaitGroup

	for port, reg := range portMap {
		addr := fmt.Sprintf(":%d", port)
		handler := NewProxyHandler(reg)
		server := newServer(addr, handler)
		servers = append(servers, server)

		wg.Add(1)
		go func(s *http.Server, prefix, addr string) {
			defer wg.Done()
			if cfg.TLSCert != "" && cfg.TLSKey != "" {
				log.Printf("✅ 启动 (HTTPS) %s -> %s", addr, prefix)
				if err := s.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != http.ErrServerClosed {
					log.Printf("❌ 服务 %s 异常: %v", addr, err)
				}
			} else {
				log.Printf("✅ 启动 (HTTP) %s -> %s", addr, prefix)
				if err := s.ListenAndServe(); err != http.ErrServerClosed {
					log.Printf("❌ 服务 %s 异常: %v", addr, err)
				}
			}
		}(server, reg.Prefix, addr)
	}

	go handleSignals(func() {
		log.Println("正在关闭所有服务...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, s := range servers {
			s.Shutdown(ctx)
		}
	})

	wg.Wait()
}

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}
}

func handleSignals(onShutdown func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	onShutdown()
}

// --- daemon ---

func runDaemon() {
	var args []string
	for _, a := range os.Args[1:] {
		if a != "-d" {
			args = append(args, a)
		}
	}
	proc, err := os.StartProcess(os.Args[0], append([]string{os.Args[0]}, args...), &os.ProcAttr{
		Dir:   ".",
		Env:   append(os.Environ(), "_DOCKER_PROXY_CHILD=1"),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Sys:   &syscall.SysProcAttr{Setsid: true},
	})
	if err != nil {
		log.Fatalf("启动守护进程失败: %v", err)
	}
	fmt.Printf("Docker 代理已在后台启动, PID: %d\n", proc.Pid)
	if f, err := os.Create("docker-proxy.pid"); err == nil {
		fmt.Fprintf(f, "%d\n", proc.Pid)
		f.Close()
	}
	os.Exit(0)
}

func setupLogging() {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("无法打开日志文件: %v", err)
	}
	log.SetOutput(f)
}
