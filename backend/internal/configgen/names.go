package configgen

import (
	"fluxor/internal/config"
)

// subNames 提取所有订阅名称
func subNames(subs []config.Subscription) []string {
	names := make([]string, len(subs))
	for i, s := range subs {
		names[i] = s.Name
	}
	return names
}
