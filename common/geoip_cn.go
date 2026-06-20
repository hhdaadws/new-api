package common

import (
	_ "embed"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
)

// chnroutesData 内嵌中国大陆 IP 段数据集(IPv4 + IPv6 CIDR 列表),
// 编译进二进制,运行时无需联网即可判断 IP 是否归属中国大陆。
//
//go:embed chnroutes.txt
var chnroutesData string

type ipRange4 struct {
	start uint32
	end   uint32
}

type ipRange6 struct {
	start netip.Addr
	end   netip.Addr
}

var (
	cnRanges4 []ipRange4
	cnRanges6 []ipRange6
)

func init() {
	loadChinaRoutes()
}

func loadChinaRoutes() {
	lines := strings.Split(chnroutesData, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			continue
		}
		prefix = prefix.Masked()
		addr := prefix.Addr()
		if addr.Is4() {
			start := beUint32(addr.As4())
			end := start | (^uint32(0) >> uint(prefix.Bits()))
			cnRanges4 = append(cnRanges4, ipRange4{start: start, end: end})
		} else {
			cnRanges6 = append(cnRanges6, ipRange6{start: addr, end: lastAddr6(prefix)})
		}
	}

	sort.Slice(cnRanges4, func(i, j int) bool {
		return cnRanges4[i].start < cnRanges4[j].start
	})
	sort.Slice(cnRanges6, func(i, j int) bool {
		return cnRanges6[i].start.Compare(cnRanges6[j].start) < 0
	})

	if len(cnRanges4) == 0 && len(cnRanges6) == 0 {
		SysError("china mainland IP dataset is empty, geo-block will not match any IP")
	} else {
		SysLog(fmt.Sprintf("china mainland IP dataset loaded: %d IPv4 ranges, %d IPv6 ranges", len(cnRanges4), len(cnRanges6)))
	}
}

func beUint32(b [4]byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// lastAddr6 返回 IPv6 前缀的最后一个地址(主机位全置 1)。
func lastAddr6(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Addr().As16()
	for i := prefix.Bits(); i < 128; i++ {
		bytes[i/8] |= 1 << (7 - uint(i)%8)
	}
	return netip.AddrFrom16(bytes)
}

// IsChinaMainlandIP 判断给定 IP 是否归属中国大陆。
// 无法解析的 IP 一律返回 false(不拦截),以避免误伤。
func IsChinaMainlandIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()

	if addr.Is4() {
		v := beUint32(addr.As4())
		idx := sort.Search(len(cnRanges4), func(i int) bool {
			return cnRanges4[i].start > v
		})
		if idx == 0 {
			return false
		}
		return v <= cnRanges4[idx-1].end
	}

	idx := sort.Search(len(cnRanges6), func(i int) bool {
		return cnRanges6[i].start.Compare(addr) > 0
	})
	if idx == 0 {
		return false
	}
	return addr.Compare(cnRanges6[idx-1].end) <= 0
}

// IsChinaMainlandIPString 是 IsChinaMainlandIP 的字符串封装。
func IsChinaMainlandIPString(s string) bool {
	return IsChinaMainlandIP(net.ParseIP(s))
}
