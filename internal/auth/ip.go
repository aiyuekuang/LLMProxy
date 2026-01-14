package auth

import (
	"net"
)

// CheckIPAllowed 检查 IP 是否允许访问
// 参数：
//   - clientIP: 客户端 IP
//   - allowedIPs: IP 白名单（CIDR 格式）
//   - deniedIPs: IP 黑名单（CIDR 格式）
// 返回：
//   - bool: 是否允许
func CheckIPAllowed(clientIP string, allowedIPs []string, deniedIPs []string) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	
	// 1. 检查黑名单（优先）
	for _, cidr := range deniedIPs {
		if matchCIDR(ip, cidr) {
			return false
		}
	}
	
	// 2. 检查白名单
	// 如果白名单为空，表示允许所有 IP
	if len(allowedIPs) == 0 {
		return true
	}
	
	for _, cidr := range allowedIPs {
		if matchCIDR(ip, cidr) {
			return true
		}
	}
	
	return false
}

// matchCIDR 检查 IP 是否匹配 CIDR
// 参数：
//   - ip: IP 地址
//   - cidr: CIDR 字符串
// 返回：
//   - bool: 是否匹配
func matchCIDR(ip net.IP, cidr string) bool {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		// 如果不是 CIDR 格式，尝试作为单个 IP 匹配
		if ip.String() == cidr {
			return true
		}
		return false
	}
	
	return ipNet.Contains(ip)
}
