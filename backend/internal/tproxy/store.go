package tproxy

import (
	"encoding/json"
	"fluxor/internal/config"
	"os"
	"path/filepath"
)

// LoadTproxyDstExceptions 加载目的例外
func LoadTproxyDstExceptions() []string {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()
	data, err := os.ReadFile(config.FluxorConfigFile)
	if err != nil {
		return []string{"# 公共 DNS 服务器", "223.5.5.5 #注释可单独一行也可写在规则后", "1.12.12.12", "# stun服务器", "141.101.90.1"} // 默认值
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []string{"# 公共 DNS 服务器", "223.5.5.5 #注释可单独一行也可写在规则后", "1.12.12.12", "# stun服务器", "141.101.90.1"}
	}
	// 尝试读取新字段 tproxy_dst_exceptions
	if dstRaw, ok := raw["tproxy_dst_exceptions"]; ok {
		var dst []string
		if err := json.Unmarshal(dstRaw, &dst); err == nil {
			tproxyDstExceptionsCache = dst
			return dst
		}
	}
	// 回退到旧字段 tproxy_exceptions
	if oldRaw, ok := raw["tproxy_exceptions"]; ok {
		var old []string
		if err := json.Unmarshal(oldRaw, &old); err == nil {
			// 迁移到新字段
			tproxyDstExceptionsCache = old
			saveTproxyDstExceptionsLocked(old)
			// 删除旧字段（可选）
			delete(raw, "tproxy_exceptions")
			return old
		}
	}
	// 默认值
	defaultDst := []string{"# 公共 DNS 服务器", "223.5.5.5 #注释可单独一行也可写在规则后", "1.12.12.12", "# stun服务器", "141.101.90.1"}
	tproxyDstExceptionsCache = defaultDst
	saveTproxyDstExceptionsLocked(defaultDst)
	return defaultDst
}

// saveTproxyDstExceptionsLocked 假定已持有锁
func saveTproxyDstExceptionsLocked(dst []string) error {
	data, err := os.ReadFile(config.FluxorConfigFile)
	var full map[string]interface{}
	if err == nil && len(data) > 0 {
		json.Unmarshal(data, &full)
	} else {
		full = make(map[string]interface{})
	}
	full["tproxy_dst_exceptions"] = dst
	// 删除旧字段（可选）
	delete(full, "tproxy_exceptions")
	newData, _ := json.MarshalIndent(full, "", "  ")
	return os.WriteFile(config.FluxorConfigFile, newData, 0644)
}

// SaveTproxyDstExceptions 供外部调用（加锁）
func SaveTproxyDstExceptions(dst []string) error {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()
	tproxyDstExceptionsCache = dst
	return saveTproxyDstExceptionsLocked(dst)
}

// loadTproxyDstExceptions 加载源例外
func LoadTproxySrcExceptions() []string {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()
	data, err := os.ReadFile(config.FluxorConfigFile)
	if err != nil {
		// 文件不存在，创建默认
		defaultSrc := []string{"# Docker 默认网段", "172.17.0.0/16"}
		tproxySrcExceptionsCache = defaultSrc
		saveTproxySrcExceptionsLocked(defaultSrc)
		return defaultSrc
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		defaultSrc := []string{"# Docker 默认网段", "172.17.0.0/16"}
		tproxySrcExceptionsCache = defaultSrc
		saveTproxySrcExceptionsLocked(defaultSrc)
		return defaultSrc
	}
	if srcRaw, ok := raw["tproxy_src_exceptions"]; ok {
		var src []string
		if err := json.Unmarshal(srcRaw, &src); err == nil {
			tproxySrcExceptionsCache = src
			return src
		}
	}
	// 字段不存在，初始化默认
	defaultSrc := []string{"# Docker 默认网段", "172.17.0.0/16"}
	tproxySrcExceptionsCache = defaultSrc
	saveTproxySrcExceptionsLocked(defaultSrc)
	return defaultSrc
}

func saveTproxySrcExceptionsLocked(src []string) error {
	data, _ := os.ReadFile(config.FluxorConfigFile)
	var full map[string]interface{}
	if len(data) > 0 {
		json.Unmarshal(data, &full)
	} else {
		full = make(map[string]interface{})
	}
	full["tproxy_src_exceptions"] = src
	newData, _ := json.MarshalIndent(full, "", "  ")
	return os.WriteFile(config.FluxorConfigFile, newData, 0644)
}

// SaveTproxySrcExceptions 保存源例外列表（加锁），由 HTTP 层调用。
func SaveTproxySrcExceptions(src []string) error {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()
	tproxySrcExceptionsCache = src
	return saveTproxySrcExceptionsLocked(src)
}

// LoadTproxyProxyLocal 从 fluxor.json 读取 tproxy_proxy_local 字段，默认 true
func LoadTproxyProxyLocal() bool {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()

	data, err := os.ReadFile(config.FluxorConfigFile)
	if err != nil {
		// 文件不存在，默认开启并保存
		tproxyProxyLocal = true
		saveTproxyProxyLocalLocked(true)
		return true
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		tproxyProxyLocal = true
		saveTproxyProxyLocalLocked(true)
		return true
	}
	var enabled bool
	if val, ok := raw["tproxy_proxy_local"]; ok {
		if err := json.Unmarshal(val, &enabled); err != nil {
			tproxyProxyLocal = true
			saveTproxyProxyLocalLocked(true)
			return true
		}
		tproxyProxyLocal = enabled
		return enabled
	}
	// 字段不存在，默认 true，写入文件
	tproxyProxyLocal = true
	saveTproxyProxyLocalLocked(true)
	return true
}

// saveTproxyProxyLocalLocked 假定已持有 exceptionsMu 锁
func saveTproxyProxyLocalLocked(enabled bool) error {
	// 读取完整配置
	data, err := os.ReadFile(config.FluxorConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var full map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &full); err != nil {
			return err
		}
	} else {
		full = make(map[string]interface{})
	}
	full["tproxy_proxy_local"] = enabled
	dir := filepath.Dir(config.FluxorConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	newData, err := json.MarshalIndent(full, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.FluxorConfigFile, newData, 0644)
}

// SaveTproxyProxyLocal 外部调用，加锁并保存
func SaveTproxyProxyLocal(enabled bool) error {
	exceptionsMu.Lock()
	defer exceptionsMu.Unlock()
	tproxyProxyLocal = enabled
	return saveTproxyProxyLocalLocked(enabled)
}
