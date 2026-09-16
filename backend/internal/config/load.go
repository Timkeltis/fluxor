package config

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// LoadSubscribeConfig 从文件加载订阅配置（启动时调用），若失败则设置默认值
func LoadSubscribeConfig() {
	Mu.Lock()
	defer Mu.Unlock()

	defaultCfg := SubscribeConfig{
		ProxyPort:      7890,
		PanelPort:      9090,
		PanelSecret:    "",
		RuleGroup:      "base",
		UIPanel:        "metacubexd",
		MetaBackendURL: "",
		Subscriptions:  []Subscription{},
		TproxyPort:     7898,
	}

	data, err := os.ReadFile(FluxorConfigFile)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("读取订阅配置失败: %v", err)
		}
		Current = defaultCfg
		return
	}

	var tmp SubscribeConfig
	if err := json.Unmarshal(data, &tmp); err != nil {
		log.Printf("解析订阅配置失败: %v，使用默认配置", err)
		Current = defaultCfg
		return
	}

	// 检查 JSON 中哪些键实际存在
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		Current = defaultCfg
		return
	}

	// 仅当键不存在时，才使用默认值（避免覆盖用户设置的 0）
	if _, ok := raw["proxy_port"]; !ok {
		tmp.ProxyPort = defaultCfg.ProxyPort
	}
	if _, ok := raw["panel_port"]; !ok {
		tmp.PanelPort = defaultCfg.PanelPort
	}
	if _, ok := raw["tproxy_port"]; !ok {
		tmp.TproxyPort = defaultCfg.TproxyPort
	}

	// 字符串类型字段：若为空则设为默认值（合理）
	if tmp.UIPanel == "" {
		tmp.UIPanel = defaultCfg.UIPanel
	}
	if tmp.Mode == "" {
		tmp.Mode = "merge"
	}
	if tmp.Subscriptions == nil {
		tmp.Subscriptions = []Subscription{}
	}

	Current = tmp
	log.Printf("成功加载订阅配置：%d 个订阅", len(Current.Subscriptions))
}

// SaveSubscribeConfig 保存订阅配置到文件
func SaveSubscribeConfig() error {
	Mu.Lock()
	defer Mu.Unlock()
	data, err := json.MarshalIndent(Current, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(FluxorConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(FluxorConfigFile, data, 0644)
}
