package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// --- HTTP clients ---

var registryClient = &http.Client{
	Timeout: 300 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:       &tls.Config{},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var downloadClient = &http.Client{
	Timeout: 600 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:       &tls.Config{},
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 120 * time.Second,
	},
}

// --- regex ---

var (
	v2ShortPathRegex = regexp.MustCompile(`^/v2/[^/]+/[^/]+/[^/]+$`)
	v2LibraryRegex   = regexp.MustCompile(`^/v2/library`)
)

// --- ProxyHandler ---

type ProxyHandler struct {
	registry *RegistryConfig
}

// NewProxyHandler 创建代理 handler
func NewProxyHandler(registry *RegistryConfig) *ProxyHandler {
	return &ProxyHandler{registry: registry}
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[%s:%d] %s %s%s", r.RemoteAddr, h.getPort(), r.Method, r.URL.Path, qstr(r))

	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,TRACE,DELETE,HEAD,OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "1728000")
		w.WriteHeader(http.StatusOK)
		return
	}

	ua := strings.ToLower(r.Header.Get("User-Agent"))
	path := r.URL.Path

	// 浏览器访问 / 时显示搜索页
	if path == "/" && strings.Contains(ua, "mozilla") {
		if h.registry.Prefix == "docker.io" {
			serveSearchPage(w)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "Docker Registry Proxy → %s", h.registry.Prefix)
		}
		return
	}

	// /v2/ ping
	if path == "/v2/" || path == "/v2" {
		h.handleV2Ping(w)
		return
	}

	// /health
	if path == "/health" {
		h.handleHealth(w)
		return
	}

	// /search 搜索镜像（仅 docker.io）
	if path == "/search" && h.registry.Prefix == "docker.io" {
		h.handleSearch(w, r)
		return
	}

	// V2 API 代理
	if strings.HasPrefix(path, "/v2/") {
		h.handleV2(w, r)
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

func (h *ProxyHandler) getPort() int {
	if h.registry.Port > 0 {
		return h.registry.Port
	}
	return 5000
}

// handleV2Ping /v2/ 端点（返回 200 表示支持 Registry V2 API）
func (h *ProxyHandler) handleV2Ping(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}"))
}

// handleHealth 健康检查
func (h *ProxyHandler) handleHealth(w http.ResponseWriter) {
	if len(h.registry.Mirrors) == 0 {
		http.Error(w, "no mirrors configured", http.StatusServiceUnavailable)
		return
	}

	mirror := h.registry.Mirrors[0]
	checkURL := mirror.Host + "/v2/"

	start := time.Now()
	req, _ := http.NewRequest("GET", checkURL, nil)
	req.Header.Set("User-Agent", "docker-proxy/health-check")
	resp, err := registryClient.Do(req)
	elapsed := time.Since(start)

	status := "OK"
	code := 0
	if err != nil {
		status = "FAIL: " + err.Error()
	} else {
		resp.Body.Close()
		code = resp.StatusCode
		if code >= 500 {
			status = fmt.Sprintf("FAIL: HTTP %d", code)
		} else {
			status = fmt.Sprintf("OK (HTTP %d)", code)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"registry":"%s","mirror":"%s","status":"%s","latency":"%s"}`,
		h.registry.Prefix, mirror.Host, status, elapsed.Round(time.Millisecond))
}

// handleSearch 代理 Docker Hub 搜索 API 并渲染结果页面
func (h *ProxyHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	// 尝试通过 hub.docker.com v2 API 搜索（需要海外服务器）
	searchURL := fmt.Sprintf("https://hub.docker.com/v2/search/repositories/?query=%s&page_size=25", query)
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// 模拟浏览器请求头（降低被 Cloudflare 拦截概率）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://hub.docker.com/search")
	req.Close = true

	resp, err := downloadClient.Do(req)
	if err != nil {
		log.Printf("搜索请求失败: %v (确保服务部署在可访问 hub.docker.com 的海外服务器)", err)
		// 降级：返回带提示的空结果页
		serveSearchResultPage(w, query, nil)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("搜索 API 返回 %d: %s", resp.StatusCode, string(body))
		serveSearchResultPage(w, query, nil)
		return
	}

	// 解析 v2 API 响应
	var searchResult struct {
		Results []struct {
			RepoName    string `json:"repo_name"`
			ShortDesc   string `json:"short_description"`
			StarCount   int    `json:"star_count"`
			PullCount   int64  `json:"pull_count"`
			RepoOwner   string `json:"repo_owner"`
			IsOfficial  bool   `json:"is_official"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &searchResult); err != nil {
		log.Printf("解析搜索结果失败: %v", err)
		serveSearchResultPage(w, query, nil)
		return
	}

	// 转换为统一格式
	var summaries []SearchSummary
	for _, item := range searchResult.Results {
		pullCount := fmt.Sprintf("%d", item.PullCount)
		if item.PullCount > 1e9 {
			pullCount = fmt.Sprintf("%.1fB+", float64(item.PullCount)/1e9)
		} else if item.PullCount > 1e6 {
			pullCount = fmt.Sprintf("%.1fM+", float64(item.PullCount)/1e6)
		} else if item.PullCount > 1e3 {
			pullCount = fmt.Sprintf("%.1fK+", float64(item.PullCount)/1e3)
		}
		
		summaries = append(summaries, SearchSummary{
			Name:      item.RepoName,
			Slug:      item.RepoName,
			ShortDesc: item.ShortDesc,
			StarCount: item.StarCount,
			PullCount: pullCount,
		})
	}

	// 渲染搜索结果页面
	serveSearchResultPage(w, query, summaries)
}

// handleV2 处理 V2 API 请求
func (h *ProxyHandler) handleV2(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Docker Hub 官方镜像自动补 library/ 前缀
	if h.registry.Prefix == "docker.io" && v2ShortPathRegex.MatchString(path) && !v2LibraryRegex.MatchString(path) {
		if parts := strings.SplitN(path, "/v2/", 2); len(parts) == 2 {
			path = "/v2/library/" + parts[1]
			log.Printf("补全 library/: %s -> %s", r.URL.Path, path)
		}
	}

	if len(h.registry.Mirrors) == 0 {
		http.Error(w, "no mirrors configured", http.StatusServiceUnavailable)
		return
	}

	mirror := h.registry.Mirrors[0]
	target := mirror.Host + path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	// 尝试无 token 请求，如果 401 则获取 token 重试
	clientAuth := r.Header.Get("Authorization")
	h.proxyWithAuth(w, r, target, "", clientAuth)
}

// proxyWithAuth 转发请求，如果 401 则自动获取 token 并重试一次
func (h *ProxyHandler) proxyWithAuth(w http.ResponseWriter, origReq *http.Request, target, token, clientAuth string) {
	req, err := http.NewRequest(origReq.Method, target, origReq.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copySelectHeaders(req.Header, origReq.Header)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if clientAuth != "" {
		req.Header.Set("Authorization", clientAuth)
	}

	// 应用 mirror header（如私有 registry 认证）
	if len(h.registry.Mirrors) > 0 {
		for k, v := range h.registry.Mirrors[0].Header {
			req.Header.Set(k, v)
		}
	}

	resp, err := registryClient.Do(req)
	if err != nil {
		log.Printf("上游请求失败: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 如果返回 401 且有 WWW-Authenticate，则获取 token 重试
	if resp.StatusCode == http.StatusUnauthorized && token == "" {
		if wwwAuth := resp.Header.Get("WWW-Authenticate"); wwwAuth != "" {
			log.Printf("收到 401，尝试获取 token (WWW-Authenticate: %s)", wwwAuth)
			if challenge := parseWWWAuthenticate(wwwAuth); challenge != nil {
				newToken, err := fetchToken(challenge, clientAuth)
				if err != nil {
					log.Printf("获取 token 失败: %v", err)
				} else {
					// 重新发起请求（带 token）
					log.Printf("重试请求（已获取 token）")
					// 需要重置 body（如果有）
					if origReq.Body != nil {
						// 实际场景中 GET/HEAD 通常没有 body，如有需要可实现 body 缓存
						origReq.Body = http.NoBody
					}
					h.proxyWithAuth(w, origReq, target, newToken, clientAuth)
					return
				}
			}
		}
	}

	// 处理重定向（blob 下载时常见）
	if loc := resp.Header.Get("Location"); loc != "" && isRedirectCode(resp.StatusCode) {
		log.Printf("跟随重定向: %s", loc)
		h.handleCDNRedirect(w, origReq, loc)
		return
	}

	flushResponse(w, resp)
}

// handleCDNRedirect 跟随 CDN 重定向下载 blob
func (h *ProxyHandler) handleCDNRedirect(w http.ResponseWriter, origReq *http.Request, location string) {
	req, err := http.NewRequest(origReq.Method, location, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	copyAllHeaders(req.Header, origReq.Header)
	req.Header.Del("Authorization") // CDN 通常不需要 auth

	resp, err := downloadClient.Do(req)
	if err != nil {
		log.Printf("CDN 下载失败: %v", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Access-Control-Expose-Headers", "*")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "max-age=1500")
	w.Header().Del("Content-Security-Policy")
	w.Header().Del("Content-Security-Policy-Report-Only")
	w.Header().Del("Clear-Site-Data")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// --- helpers ---

func copySelectHeaders(dst, src http.Header) {
	for _, k := range []string{
		"User-Agent", "Accept", "Accept-Language", "Accept-Encoding",
		"Connection", "Cache-Control", "If-None-Match", "If-Modified-Since",
	} {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}

func copyAllHeaders(dst, src http.Header) {
	for k, vv := range src {
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func flushResponse(w http.ResponseWriter, resp *http.Response) {
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Expose-Headers", "*")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func isRedirectCode(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

func qstr(r *http.Request) string {
	if r.URL.RawQuery != "" {
		return "?" + r.URL.RawQuery
	}
	return ""
}
