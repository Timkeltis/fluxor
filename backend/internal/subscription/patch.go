package subscription

import (
	"fluxor/internal/config"
	"fluxor/internal/configgen"
	"fmt"
	"os"
	"regexp"
)

// patchSubscriptionFile 修改订阅文件，添加/更新必要字段
func patchSubscriptionFile(filePath string, cfg config.SubscribeConfig) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	text := string(content)

	// 清理可能冲突的顶层单端口定义，防止重复端口绑定导致内核崩溃
	conflictKeys := []string{"port", "socks-port", "redir-port"}
	for _, k := range conflictKeys {
		reConflict := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(k) + `:\s*\d+\s*$`)
		text = reConflict.ReplaceAllString(text, "# "+k+" removed by Fluxor")
	}

	// 定义待替换/添加的字段
	rules := []struct {
		key   string
		value string
	}{
		{"mixed-port", fmt.Sprintf("%d", cfg.ProxyPort)},
		{"tproxy-port", fmt.Sprintf("%d", cfg.TproxyPort)},
		{"external-controller", fmt.Sprintf("'0.0.0.0:%d'", cfg.PanelPort)},
		{"external-controller-unix", fmt.Sprintf("'%s'", config.CoreSocket)},
		{"secret", fmt.Sprintf("'%s'", cfg.PanelSecret)},
		{"allow-lan", "true"},
		{"ipv6", "true"},
		{"unified-delay", "true"},
		{"geodata-mode", "false"},
		{"routing-mark", "255"},
	}

	// 根据面板选择 external-ui
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

	// 遍历规则，替换或追加
	for _, r := range rules {
		// 匹配行首的键，后跟冒号和任意空白
		re := regexp.MustCompile(`(?m)^(` + regexp.QuoteMeta(r.key) + `):\s*.*$`)
		if re.MatchString(text) {
			// 替换已有行
			text = re.ReplaceAllString(text, "$1: "+r.value)
		} else {
			// 不存在则追加到文件末尾
			text += "\n" + r.key + ": " + r.value
		}
	}

	// ----- DNS 监听注入，替换/注入完整 DNS 配置 -----
	reDns := regexp.MustCompile(`(?m)^dns:\s*\r?\n((?:[ \t]+.*\r?\n?)*)`)
	text = reDns.ReplaceAllString(text, "")
	text += "\n" + configgen.DnsBlock + "\n"

	return os.WriteFile(filePath, []byte(text), 0644)
}
