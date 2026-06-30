package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// --- token 缓存 ---

type tokenEntry struct {
	token   string
	expires time.Time
}

var (
	tokenCacheMu sync.RWMutex
	tokenCache   = make(map[string]tokenEntry)
)

func getCachedToken(key string) (string, bool) {
	tokenCacheMu.RLock()
	defer tokenCacheMu.RUnlock()
	if e, ok := tokenCache[key]; ok && time.Now().Before(e.expires) {
		return e.token, true
	}
	return "", false
}

func setCachedToken(key, token string, ttl time.Duration) {
	tokenCacheMu.Lock()
	defer tokenCacheMu.Unlock()
	tokenCache[key] = tokenEntry{token: token, expires: time.Now().Add(ttl)}
}

// authChallenge WWW-Authenticate 头解析结果
type authChallenge struct {
	Realm   string
	Service string
	Scope   string
}

// parseWWWAuthenticate 解析上游返回的 WWW-Authenticate 头
// 形如: Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"
func parseWWWAuthenticate(header string) *authChallenge {
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return nil
	}
	c := &authChallenge{}
	params := header[len("Bearer "):]
	for _, part := range splitAuthParams(params) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
		switch key {
		case "realm":
			c.Realm = val
		case "service":
			c.Service = val
		case "scope":
			c.Scope = val
		}
	}
	if c.Realm == "" {
		return nil
	}
	return c
}

// splitAuthParams 按逗号拆分，但忽略引号内的逗号
func splitAuthParams(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ',' && !inQuote:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// fetchToken 向 auth 服务请求 token
// clientAuth 为客户端透传的 Authorization 头（用于 docker login 私有镜像），可为空（匿名拉取）
func fetchToken(c *authChallenge, clientAuth string) (string, error) {
	cacheKey := c.Realm + "|" + c.Service + "|" + c.Scope + "|" + clientAuth
	if t, ok := getCachedToken(cacheKey); ok {
		return t, nil
	}

	tokenURL := c.Realm
	q := url.Values{}
	if c.Service != "" {
		q.Set("service", c.Service)
	}
	if c.Scope != "" {
		q.Set("scope", c.Scope)
	}
	if enc := q.Encode(); enc != "" {
		if strings.Contains(tokenURL, "?") {
			tokenURL += "&" + enc
		} else {
			tokenURL += "?" + enc
		}
	}

	req, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "docker-proxy/1.0")
	// 透传客户端凭证（docker login）
	if clientAuth != "" {
		req.Header.Set("Authorization", clientAuth)
	}

	resp, err := registryClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("auth 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 token 失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth 服务返回 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析 token 失败: %w", err)
	}

	token := result.Token
	if token == "" {
		token = result.AccessToken
	}
	if token == "" {
		return "", fmt.Errorf("token 为空, resp=%s", string(body))
	}

	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 0 || ttl > 300*time.Second {
		ttl = 250 * time.Second
	} else {
		ttl -= 30 * time.Second
	}
	setCachedToken(cacheKey, token, ttl)
	log.Printf("token 已缓存 (scope=%s, ttl=%s)", c.Scope, ttl)
	return token, nil
}
