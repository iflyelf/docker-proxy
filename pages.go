package main

import (
	"fmt"
	"html"
	"net/http"
)

// SearchSummary Docker Hub 搜索结果项
type SearchSummary struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ShortDesc string `json:"short_description"`
	StarCount int    `json:"star_count"`
	PullCount string `json:"pull_count"`
}

func serveNginxPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>Welcome to nginx!</title>
<style>body{width:35em;margin:0 auto;font-family:Tahoma,Verdana,Arial,sans-serif;}</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and working. Further configuration is required.</p>
<p>For online documentation and support please refer to <a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at <a href="http://nginx.com/">nginx.com</a>.</p>
<p><em>Thank you for using nginx.</em></p>
</body></html>`)
}

func serveSearchPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<title>Docker Hub 镜像搜索</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
	<style>
	:root {
		--primary-color: #0066ff;
		--primary-dark: #0052cc;
		--gradient-start: #1a90ff;
		--gradient-end: #003eb3;
		--text-color: #ffffff;
		--transition-time: 0.3s;
	}
	* { box-sizing: border-box; margin: 0; padding: 0; }
	body {
		font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
		display: flex; flex-direction: column; justify-content: center; align-items: center;
		min-height: 100vh; margin: 0;
		background: linear-gradient(135deg, var(--gradient-start) 0%, var(--gradient-end) 100%);
		padding: 20px; color: var(--text-color); overflow-x: hidden;
	}
	.container {
		text-align: center; width: 100%; max-width: 800px; padding: 20px; margin: 0 auto;
		display: flex; flex-direction: column; justify-content: center; min-height: 60vh;
		animation: fadeIn 0.8s ease-out;
	}
	@keyframes fadeIn { from { opacity: 0; transform: translateY(20px); } to { opacity: 1; transform: translateY(0); } }
	.title {
		color: var(--text-color); font-size: 2.3em; margin-bottom: 10px;
		text-shadow: 0 2px 10px rgba(0,0,0,0.2); font-weight: 700; letter-spacing: -0.5px;
	}
	.subtitle {
		color: rgba(255,255,255,0.9); font-size: 1.1em; margin-bottom: 25px;
		max-width: 600px; margin-left: auto; margin-right: auto; line-height: 1.4;
	}
	.search-container {
		display: flex; align-items: stretch; width: 100%; max-width: 600px; margin: 0 auto;
		height: 55px; box-shadow: 0 10px 25px rgba(0,0,0,0.15); border-radius: 12px; overflow: hidden;
	}
	#search-input {
		flex: 1; padding: 0 20px; font-size: 16px; border: none; outline: none;
		transition: all var(--transition-time) ease; height: 100%;
	}
	#search-button {
		width: 60px; background-color: var(--primary-color); border: none; cursor: pointer;
		transition: all var(--transition-time) ease; height: 100%;
		display: flex; align-items: center; justify-content: center;
	}
	#search-button:hover { background-color: var(--primary-dark); }
	.tips { color: rgba(255,255,255,0.8); margin-top: 20px; font-size: 0.9em; }
	@media (max-width: 768px) { .title { font-size: 2em; } .search-container { height: 50px; } }
	@media (max-width: 480px) { .title { font-size: 1.7em; } .search-container { height: 45px; } #search-button { width: 50px; } }
	</style>
</head>
<body>
	<div class="container">
		<h1 class="title">Docker Hub 镜像搜索</h1>
		<p class="subtitle">快速查找、下载和部署 Docker 容器镜像</p>
		<div class="search-container">
			<input type="text" id="search-input" placeholder="输入关键词搜索镜像，如: nginx, mysql, redis...">
			<button id="search-button" title="搜索">
				<svg width="20" height="20" fill="none" stroke="white" stroke-width="2" viewBox="0 0 24 24">
					<path d="M13 5l7 7-7 7M5 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round"></path>
				</svg>
			</button>
		</div>
		<p class="tips">Docker Registry Proxy — 自建镜像代理服务</p>
	</div>
	<script>
	function performSearch() {
		const q = document.getElementById('search-input').value;
		if (q) window.location.href = '/search?q=' + encodeURIComponent(q);
	}
	document.getElementById('search-button').addEventListener('click', performSearch);
	document.getElementById('search-input').addEventListener('keypress', function(e) {
		if (e.key === 'Enter') performSearch();
	});
	window.addEventListener('load', function() { document.getElementById('search-input').focus(); });
	</script>
</body>
</html>`)
}

// serveSearchResultPage 渲染搜索结果页面（响应式 H5，服务端渲染）
func serveSearchResultPage(w http.ResponseWriter, query string, results []SearchSummary) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	escapedQuery := html.EscapeString(query)
	resultCount := len(results)

	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
	<title>搜索结果: `+escapedQuery+` - Docker Hub 镜像代理</title>
	<style>
	* { box-sizing: border-box; margin: 0; padding: 0; }
	body {
		font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
		background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		min-height: 100vh;
		padding: 20px 10px;
		color: #333;
	}
	.container {
		max-width: 900px;
		margin: 0 auto;
	}
	.header {
		background: rgba(255,255,255,0.95);
		backdrop-filter: blur(10px);
		border-radius: 16px;
		padding: 20px;
		margin-bottom: 20px;
		box-shadow: 0 8px 32px rgba(0,0,0,0.1);
	}
	.search-box {
		display: flex;
		gap: 10px;
		margin-bottom: 15px;
	}
	.search-box input {
		flex: 1;
		padding: 12px 16px;
		font-size: 16px;
		border: 2px solid #e0e0e0;
		border-radius: 8px;
		outline: none;
		transition: border-color 0.3s;
	}
	.search-box input:focus {
		border-color: #667eea;
	}
	.search-box button {
		padding: 12px 24px;
		font-size: 16px;
		background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
		color: white;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-weight: 500;
		transition: transform 0.2s, box-shadow 0.2s;
		white-space: nowrap;
	}
	.search-box button:hover {
		transform: translateY(-2px);
		box-shadow: 0 4px 12px rgba(102, 126, 234, 0.4);
	}
	.search-box button:active {
		transform: translateY(0);
	}
	.result-info {
		color: #666;
		font-size: 14px;
	}
	.result-info strong {
		color: #667eea;
		font-weight: 600;
	}
	.loading {
		text-align: center;
		padding: 40px;
		color: #667eea;
		font-size: 16px;
	}
	.results {
		display: grid;
		gap: 15px;
	}
	.result-item {
		background: rgba(255,255,255,0.95);
		backdrop-filter: blur(10px);
		border-radius: 12px;
		padding: 16px;
		transition: transform 0.3s, box-shadow 0.3s;
		box-shadow: 0 4px 12px rgba(0,0,0,0.08);
	}
	.result-item:hover {
		transform: translateY(-4px);
		box-shadow: 0 8px 24px rgba(0,0,0,0.15);
	}
	.result-header {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 10px;
	}
	.result-title {
		flex: 1;
		min-width: 0;
	}
	.result-name {
		font-size: 18px;
		font-weight: 600;
		color: #667eea;
		word-break: break-all;
		margin-bottom: 4px;
	}
	.result-desc {
		color: #666;
		font-size: 14px;
		line-height: 1.5;
		margin-bottom: 12px;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.result-meta {
		display: flex;
		gap: 16px;
		font-size: 13px;
		color: #999;
		flex-wrap: wrap;
	}
	.result-command {
		background: #f8f9fa;
		border: 1px solid #e0e0e0;
		border-radius: 6px;
		padding: 10px 12px;
		font-family: "Consolas", "Monaco", "Courier New", monospace;
		font-size: 13px;
		color: #333;
		margin-top: 10px;
		word-break: break-all;
		cursor: pointer;
		user-select: all;
	}
	.result-command:hover {
		background: #e9ecef;
	}
	.no-results {
		background: rgba(255,255,255,0.95);
		backdrop-filter: blur(10px);
		border-radius: 12px;
		padding: 60px 20px;
		text-align: center;
		box-shadow: 0 8px 32px rgba(0,0,0,0.1);
	}
	.no-results h2 {
		font-size: 24px;
		color: #667eea;
		margin-bottom: 12px;
	}
	.no-results p {
		color: #666;
		font-size: 16px;
	}
	.error-box {
		background: rgba(255,255,255,0.95);
		border-radius: 12px;
		padding: 40px 20px;
		text-align: center;
		box-shadow: 0 8px 32px rgba(0,0,0,0.1);
		color: #e74c3c;
	}
	@media (max-width: 600px) {
		body { padding: 15px 10px; }
		.header { padding: 16px; }
		.search-box { flex-direction: column; }
		.search-box button { width: 100%; }
		.result-item { padding: 14px; }
		.result-name { font-size: 16px; }
		.result-desc { font-size: 13px; }
	}
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<form class="search-box" action="/search" method="get">
				<input type="text" name="q" placeholder="搜索 Docker 镜像..." value="`+escapedQuery+`" required>
				<button type="submit">🔍 搜索</button>
			</form>
			<div class="result-info">找到 <strong>`+fmt.Sprintf("%d", resultCount)+`</strong> 个结果</div>
		</div>
		<div class="results">`)

	if resultCount == 0 {
		fmt.Fprint(w, `
			<div class="no-results">
				<h2>😔 未找到相关镜像</h2>
				<p>试试其他关键词，如: nginx、redis、mysql</p>
			</div>`)
	} else {
		for _, item := range results {
			name := html.EscapeString(item.Name)
			desc := html.EscapeString(item.ShortDesc)
			if desc == "" {
				desc = "暂无描述"
			}
			pullCount := html.EscapeString(item.PullCount)
			stars := fmt.Sprintf("%d", item.StarCount)

			// docker pull 命令
			pullCmd := "docker pull " + name

			fmt.Fprintf(w, `
			<div class="result-item">
				<div class="result-header">
					<div class="result-title">
						<div class="result-name">%s</div>
					</div>
				</div>
				<div class="result-desc">%s</div>
				<div class="result-meta">
					<span>⭐ %s</span>
					<span>📥 %s</span>
				</div>
				<div class="result-command" title="点击复制命令">%s</div>
			</div>`, name, desc, stars, pullCount, pullCmd)
		}
	}

	fmt.Fprint(w, `
		</div>
	</div>
	<script>
	document.querySelectorAll('.result-command').forEach(el => {
		el.addEventListener('click', function() {
			const range = document.createRange();
			range.selectNodeContents(this);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
		});
	});
	</script>
</body>
</html>`)
}
