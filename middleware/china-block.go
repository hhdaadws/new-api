package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/gin-gonic/gin"
)

// 中国大陆访问拦截相关环境变量:
//   BLOCK_CHINA_MAINLAND        是否启用(默认 true,设为 false 可关闭)
//   CHINA_BLOCK_IP_WHITELIST    放行的 IP / CIDR 列表(逗号分隔),即使命中大陆也放行
const (
	envBlockChinaMainland = "BLOCK_CHINA_MAINLAND"
	envChinaBlockWhitelist = "CHINA_BLOCK_IP_WHITELIST"
)

const chinaBlockMessage = "本服务不向中国大陆地区提供服务。This service is not available in mainland China."

// chinaBlockHTML 是面向浏览器(前端页面)访问时返回的提示页。
const chinaBlockHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Service Unavailable</title>
<style>
body{margin:0;height:100vh;display:flex;align-items:center;justify-content:center;background:#0f172a;color:#e2e8f0;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,"Helvetica Neue",Arial,"PingFang SC","Microsoft YaHei",sans-serif}
.box{max-width:520px;padding:40px;text-align:center;line-height:1.7}
h1{font-size:22px;margin:0 0 16px}
p{font-size:15px;color:#94a3b8;margin:6px 0}
</style>
</head>
<body>
<div class="box">
<h1>服务不可用 / Service Unavailable</h1>
<p>本服务不向中国大陆地区提供服务。</p>
<p>This service is not available in mainland China.</p>
</div>
</body>
</html>`

// ChinaMainlandBlock 拦截来自中国大陆的 IP(前端页面与 API 均覆盖)。
// 未启用时返回一个轻量的 pass-through 中间件。
func ChinaMainlandBlock() gin.HandlerFunc {
	enabled := common.GetEnvOrDefaultBool(envBlockChinaMainland, true)
	if !enabled {
		return func(c *gin.Context) { c.Next() }
	}

	var whitelist []string
	if raw := common.GetEnvOrDefaultString(envChinaBlockWhitelist, ""); raw != "" {
		for _, item := range strings.Split(raw, ",") {
			if item = strings.TrimSpace(item); item != "" {
				whitelist = append(whitelist, item)
			}
		}
	}

	common.SysLog("china mainland IP block is enabled")

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		ip := net.ParseIP(clientIP)
		if ip == nil {
			// 无法解析的 IP 放行,避免误伤
			c.Next()
			return
		}

		// 内网 / 回环地址放行,保证本机与内网管理可用
		if common.IsPrivateIP(ip) {
			c.Next()
			return
		}

		// 白名单放行
		if len(whitelist) > 0 && common.IsIpInCIDRList(ip, whitelist) {
			c.Next()
			return
		}

		if !common.IsChinaMainlandIP(ip) {
			c.Next()
			return
		}

		logger.LogInfo(c.Request.Context(), "china mainland IP blocked: "+clientIP)
		abortChinaBlocked(c)
	}
}

// abortChinaBlocked 根据请求类型返回拦截响应:
// API / relay 请求返回 JSON,浏览器页面请求返回 HTML。
func abortChinaBlocked(c *gin.Context) {
	if wantsJSON(c) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": chinaBlockMessage,
				"type":    "new_api_error",
				"code":    "region_not_supported",
			},
		})
	} else {
		c.Data(http.StatusForbidden, "text/html; charset=utf-8", []byte(chinaBlockHTML))
	}
	c.Abort()
}

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
