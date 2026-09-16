package configgen

import (
	"fluxor/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GenerateConfig 根据订阅配置生成 config.yaml
func GenerateConfig(cfg config.SubscribeConfig) error {
	// 无订阅时生成基本配置（使用配置中的端口和密钥）
	if len(cfg.Subscriptions) == 0 {
		basic := fmt.Sprintf(`mixed-port: %d
tproxy-port: %d
allow-lan: true
mode: rule
log-level: silent
external-controller-unix: '%s'
external-controller: '0.0.0.0:%d'
`, cfg.ProxyPort, cfg.TproxyPort, config.CoreSocket, cfg.PanelPort)
		if cfg.PanelSecret != "" {
			basic += fmt.Sprintf("secret: '%s'\n", cfg.PanelSecret)
		}
		uiSelect := "ui/meta"
		uiURL := ""
		if cfg.UIPanel == "zashboard" {
			uiSelect = "ui/zash"
			uiURL = `external-ui-url: "https://github.com/Zephyruso/zashboard/releases/latest/download/dist-cdn-fonts.zip"`
		}
		basic += fmt.Sprintf("external-ui: %s\n", uiSelect)
		if uiURL != "" {
			basic += uiURL + "\n"
		}
		dir := filepath.Dir(config.ConfigTarget)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		return os.WriteFile(config.ConfigTarget, []byte(basic), 0644)
	}

	// 生成 proxy-providers 子项（不包含头部，每项缩进2个空格）
	var providersBuf strings.Builder
	for i, sub := range cfg.Subscriptions {
		interval := sub.UpdateInterval
		if interval <= 0 {
			interval = 86400
		}
		health := sub.HealthInterval
		if health <= 0 {
			health = 300
		}
		prefix := sub.Prefix // 直接使用，不再依赖开关

		providersBuf.WriteString(fmt.Sprintf(`  %s:
    type: http
    url: "%s"
    interval: %d
    path: proxies/%s.yaml
    health-check:
      enable: true
      url: "https://www.gstatic.com/generate_204"
      interval: %d
`, sub.Name, sub.URL, interval, sub.Name, health))

		// 如果前缀非空，添加 override.additional-prefix
		if prefix != "" {
			providersBuf.WriteString(fmt.Sprintf(`    override:
      additional-prefix: "%s"
`, prefix))
		}

		if i < len(cfg.Subscriptions)-1 {
			providersBuf.WriteString("\n")
		}
	}

	configContent := configTemplate

	// 定义待替换/添加的字段
	rules := []struct {
		key   string
		value string
	}{
		{"mixed-port", fmt.Sprintf("%d", cfg.ProxyPort)},
		{"tproxy-port", fmt.Sprintf("%d", cfg.TproxyPort)},
		{"external-controller", fmt.Sprintf("'0.0.0.0:%d'", cfg.PanelPort)},
		{"secret", fmt.Sprintf("'%s'", cfg.PanelSecret)},
	}

	// 添加 external-ui
	uiPath := "ui/meta"
	if cfg.UIPanel == "zashboard" {
		uiPath = "ui/zash"
	}
	rules = append(rules, struct{ key, value string }{"external-ui", uiPath})

	// 仅当面板为 zashboard 时添加 external-ui-url
	if cfg.UIPanel == "zashboard" {
		rules = append(rules, struct{ key, value string }{
			"external-ui-url",
			`"https://github.com/Zephyruso/zashboard/releases/latest/download/dist-cdn-fonts.zip"`,
		})
	}

	// 应用规则到 configContent（替换或追加）
	for _, r := range rules {
		re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(r.key) + `):\s*.*$`)
		if re.MatchString(configContent) {
			configContent = re.ReplaceAllString(configContent, "$1: "+r.value)
		} else {
			configContent += "\n" + r.key + ": " + r.value
		}
	}

	// ----- 确保 proxy-providers 字段存在并注入内容 -----
	if providersBuf.Len() > 0 {
		reProxy := regexp.MustCompile(`(?m)^proxy-providers:\s*$`)
		if reProxy.MatchString(configContent) {
			// 替换主键行，并在其后插入 providers 内容（providersBuf 已含缩进）
			configContent = reProxy.ReplaceAllString(configContent, "proxy-providers:\n"+providersBuf.String())
		} else {
			// 不存在则追加
			configContent += "\nproxy-providers:\n" + providersBuf.String()
		}
	}

	// 根据规则集生成动态块
	var groupsBlock, providersBlock, rulesBlock string
	switch cfg.RuleGroup {
	case "base":
		groupsBlock = proxyGroupsBase
		providersBlock = "" // base 不生成 rule-providers
		rulesBlock = rulesBase
	case "full":
		// 替换 __SUB_NAMES__ 为订阅名称列表（逗号分隔）
		subList := strings.Join(subNames(cfg.Subscriptions), ",")
		groupsBlock = strings.ReplaceAll(proxyGroupsFullTemplate, "__SUB_NAMES__", subList)
		providersBlock = ruleProvidersFull
		rulesBlock = rulesFull
	default:
		return fmt.Errorf("未知规则集: %s", cfg.RuleGroup)
	}

	// 追加到配置末尾（顺序：rule-providers -> proxy-groups -> rules）
	if providersBlock != "" {
		configContent += "\n" + providersBlock
	}
	configContent += "\n" + groupsBlock + "\n" + rulesBlock

	// ----- DNS 监听注入 -----
	reDns := regexp.MustCompile(`(?m)^dns:\s*\r?\n((?:[ \t]+.*\r?\n)*?)`)
	configContent = reDns.ReplaceAllString(configContent, "")
	// 追加新的 DNS 块（确保前后有空行）
	configContent += "\n" + DnsBlock + "\n"

	// 清理多余空行
	configContent = regexp.MustCompile(`\n{3,}`).ReplaceAllString(configContent, "\n\n")

	// 写入文件
	dir := filepath.Dir(config.ConfigTarget)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return os.WriteFile(config.ConfigTarget, []byte(configContent), 0644)
}

// GenerateBaseConfig 生成基础配置文件（用于无订阅时）
func GenerateBaseConfig(cfg config.SubscribeConfig) error {
	basic := fmt.Sprintf(`mixed-port: %d
allow-lan: true
mode: rule
log-level: silent
external-controller-unix: '%s'
external-controller: '0.0.0.0:%d'
`, cfg.ProxyPort, config.CoreSocket, cfg.PanelPort)

	// 如果面板密钥不为空，追加 secret 字段
	if cfg.PanelSecret != "" {
		basic += fmt.Sprintf("secret: '%s'\n", cfg.PanelSecret)
	}

	dir := filepath.Dir(config.ConfigTarget)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	return os.WriteFile(config.ConfigTarget, []byte(basic), 0644)
}
