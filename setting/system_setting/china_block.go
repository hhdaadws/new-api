package system_setting

import (
	"net"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// 中国大陆访问限制模式
const (
	ChinaBlockModeOff   = "off"   // 关闭限制
	ChinaBlockModePopup = "popup" // 弹窗提示(同意后方可继续使用前端)
	ChinaBlockModeBlock = "block" // 直接拦截前端页面
)

// defaultChinaBlockContent 默认弹窗/拦截页正文。
const defaultChinaBlockContent = "我司检测到您的IP地址来源于中国大陆，此在我们所授权的可访问的服务地区以外。我司不会以任何形式存储您的信息，仅提供面向模型服务提供商的转发能力。请您务必知晓我司用户协议与您所在地区的相关法规并取得相关授权，我司不对您的任何行为负责。"

// ChinaBlockSettings 中国大陆访问限制设置。
// 仅作用于前端页面访问,不影响 API / relay 请求。
type ChinaBlockSettings struct {
	Mode      string `json:"mode"`      // off | popup | block
	Title     string `json:"title"`     // 弹窗 / 拦截页标题
	Content   string `json:"content"`   // 弹窗 / 拦截页正文
	Whitelist string `json:"whitelist"` // 放行的 IP / CIDR 列表(逗号分隔),即使命中大陆也放行
}

var defaultChinaBlockSettings = ChinaBlockSettings{
	Mode:      ChinaBlockModeOff,
	Title:     "访问地区提示",
	Content:   defaultChinaBlockContent,
	Whitelist: "",
}

func init() {
	config.GlobalConfig.Register("china_block", &defaultChinaBlockSettings)
}

func GetChinaBlockSettings() *ChinaBlockSettings {
	return &defaultChinaBlockSettings
}

// IsRestrictedIP 判断该 IP 是否应被视为受限的中国大陆 IP。
// 空 IP、内网 / 回环地址、白名单地址一律放行(返回 false)。
func (s *ChinaBlockSettings) IsRestrictedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	// 内网 / 回环地址放行,保证本机与内网管理可用
	if common.IsPrivateIP(ip) {
		return false
	}
	// 白名单放行
	if wl := s.whitelistEntries(); len(wl) > 0 && common.IsIpInCIDRList(ip, wl) {
		return false
	}
	return common.IsChinaMainlandIP(ip)
}

func (s *ChinaBlockSettings) whitelistEntries() []string {
	if strings.TrimSpace(s.Whitelist) == "" {
		return nil
	}
	var list []string
	for _, item := range strings.Split(s.Whitelist, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}
