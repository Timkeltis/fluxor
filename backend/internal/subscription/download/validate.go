package download

import (
	"strings"
)

// isValidSubscription 检查内容是否包含有效订阅标志
func isValidSubscription(content string) bool {
	// 检查是否包含 proxies: 或 proxy-providers: 或 proxy-groups:
	// 简单匹配，忽略大小写
	lower := strings.ToLower(content)
	return strings.Contains(lower, "proxies:") ||
		strings.Contains(lower, "proxy-providers:") ||
		strings.Contains(lower, "proxy-groups:")
}
