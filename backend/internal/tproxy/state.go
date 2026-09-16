package tproxy

import (
	"sync"
)

var (
	tproxyEnableState        bool
	tproxyMu                 sync.RWMutex
	exceptionsMu             sync.RWMutex
	tproxyProxyLocal         bool
	tproxyDstExceptionsCache []string
	tproxySrcExceptionsCache []string
)

// GetTproxyState 导出状态
func GetTproxyState() bool {
	tproxyMu.RLock()
	defer tproxyMu.RUnlock()
	return tproxyEnableState
}
