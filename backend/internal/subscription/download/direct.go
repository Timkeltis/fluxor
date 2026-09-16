package download

import (
	"encoding/base64"
	"fluxor/internal/config"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// tryDirectDownload 尝试直接 HTTP 下载订阅
func tryDirectDownload(sub config.Subscription, targetFile string) (updatedAt string, subInfo map[string]interface{}, err error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", sub.URL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "clash.meta")
	req.Header.Set("Accept", "text/plain, application/json, */*")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	// 检测是否为 Base64 编码
	content := string(bodyBytes)
	decodedContent, isBase64 := tryBase64Decode(bodyBytes)
	if isBase64 && decodedContent != "" {
		content = decodedContent
	}

	// 检查是否为有效订阅配置
	if !isValidSubscription(content) {
		return "", nil, fmt.Errorf("无效的订阅配置")
	}

	// 写入文件
	if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
		return "", nil, err
	}

	// 解析 subscription-userinfo 头
	subInfo = parseSubscriptionUserinfo(resp.Header.Get("subscription-userinfo"))
	updatedAt = time.Now().Format(time.RFC3339)

	return updatedAt, subInfo, nil
}

// tryBase64Decode 尝试解码 Base64，返回解码后的字符串和是否成功
func tryBase64Decode(data []byte) (string, bool) {
	// 去除可能的空白字符
	raw := strings.TrimSpace(string(data))
	// 尝试标准 Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return string(decoded), true
	}
	// 尝试 URL 编码 Base64 (替换 - _ 等)
	raw = strings.ReplaceAll(raw, "-", "+")
	raw = strings.ReplaceAll(raw, "_", "/")
	decoded, err = base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return string(decoded), true
	}
	return "", false
}

// parseSubscriptionUserinfo 解析 subscription-userinfo 头
func parseSubscriptionUserinfo(header string) map[string]interface{} {
	result := make(map[string]interface{})
	if header == "" {
		return result
	}
	parts := strings.Split(header, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "upload", "download", "total":
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				result[key] = v
			}
		case "expire":
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				result[key] = v
			}
		default:
			result[key] = val
		}
	}
	return result
}
