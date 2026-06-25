package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// ChinaMainlandBlock 根据系统设置(china_block.mode)拦截来自中国大陆的前端页面访问。
//
//   - off   : 不做任何限制
//   - popup : 由前端弹窗提示并要求同意,中间件直接放行(弹窗逻辑在前端实现)
//   - block : 对来自中国大陆的浏览器页面请求返回拦截页
//
// 任何模式下都不会拦截 API / relay 请求(/v1、/api、/dashboard、/mj、/pg 等),
// 即"其他 api 的请求不拦截"。
func ChinaMainlandBlock() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := system_setting.GetChinaBlockSettings()

		// 仅 block 模式在服务端拦截;off / popup 一律放行
		if s.Mode != system_setting.ChinaBlockModeBlock {
			c.Next()
			return
		}

		// API / relay 等接口请求一律放行,只拦截浏览器前端页面
		if wantsJSON(c) {
			c.Next()
			return
		}

		ip := net.ParseIP(c.ClientIP())
		if ip == nil || !s.IsRestrictedIP(ip) {
			c.Next()
			return
		}

		logger.LogInfo(c.Request.Context(), "china mainland IP blocked (frontend): "+c.ClientIP())
		c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(buildChinaBlockHTML(s)))
		c.Abort()
	}
}

// buildChinaBlockHTML 根据设置生成拦截提示页。
func buildChinaBlockHTML(s *system_setting.ChinaBlockSettings) string {
	title := s.Title
	if strings.TrimSpace(title) == "" {
		title = "访问地区提示"
	}
	content := s.Content
	if strings.TrimSpace(content) == "" {
		content = "本服务暂不向您所在的地区提供服务。"
	}
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + htmlEscape(title) + `</title>
<style>
body{margin:0;height:100vh;display:flex;align-items:center;justify-content:center;background:#0f172a;color:#e2e8f0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"PingFang SC","Microsoft YaHei",sans-serif}
.box{max-width:560px;padding:40px;text-align:center;line-height:1.8}
h1{font-size:22px;margin:0 0 16px}
p{font-size:15px;color:#94a3b8;margin:6px 0}
</style>
</head>
<body>
<div class="box">
<h1>` + htmlEscape(title) + `</h1>
<p>` + htmlEscape(content) + `</p>
</div>
</body>
</html>`
}

// htmlEscape 对用户可配置文本做最小化的 HTML 转义,避免注入。
func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

// wantsJSON 判断请求是否为 API / relay 类接口请求(返回 true 表示不应拦截前端页面)。
func wantsJSON(c *gin.Context) bool {
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/v1") ||
		strings.HasPrefix(path, "/api") ||
		strings.HasPrefix(path, "/dashboard") ||
		strings.HasPrefix(path, "/mj") ||
		strings.HasPrefix(path, "/pg") {
		return true
	}
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}
