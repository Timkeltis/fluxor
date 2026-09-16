package netinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// fetchPublicIP 支持通过代理获取 IP，兼容 JSON 与纯文本，带正则表达式提取和校验
func fetchPublicIP(apiURL, proxyAddr string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	if proxyAddr != "" {
		proxyURL, err := url.Parse(proxyAddr)
		if err == nil {
			client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		}
	}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := strings.TrimSpace(string(bodyBytes))

	// 1. 尝试解析为 JSON
	var data struct {
		IP    string `json:"ip"`
		Query string `json:"query"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err == nil {
		if data.IP != "" {
			return data.IP, nil
		}
		if data.Query != "" {
			return data.Query, nil
		}
	}

	// 2. 正则从响应内容中搜索首个合法的 IPv4/IPv6 地址并验证
	ipRegex := regexp.MustCompile(`((?:[0-9]{1,3}\.){3}[0-9]{1,3})|((?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4})`)
	matches := ipRegex.FindAllString(bodyStr, -1)
	for _, match := range matches {
		if net.ParseIP(match) != nil {
			return match, nil
		}
	}

	// 3. 直接验证去除空白后的全文
	if net.ParseIP(bodyStr) != nil {
		return bodyStr, nil
	}

	return "", fmt.Errorf("no valid IP address found in response: %s", bodyStr)
}

// fetchPublicIPWithFallback 依次尝试一组 URL，返回首个成功获取到的 IP 地址
func fetchPublicIPWithFallback(urls []string, proxyAddr string) (string, error) {
	var lastErr error
	for _, u := range urls {
		ip, err := fetchPublicIP(u, proxyAddr)
		if err == nil && ip != "" {
			return ip, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("URL 列表为空")
}
