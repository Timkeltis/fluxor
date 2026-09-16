package delaytest

import (
	"encoding/json"
	"fluxor/internal/httpx"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HandleDelayTestGoogle 测试 Google 延迟
func HandleDelayTestGoogle(w http.ResponseWriter, r *http.Request) {
	handleDelayTestCommon(w, r, "https://www.gstatic.com/generate_204")
}

// HandleDelayTestYouTube 测试 YouTube 延迟
func HandleDelayTestYouTube(w http.ResponseWriter, r *http.Request) {
	handleDelayTestCommon(w, r, "https://www.youtube.com")
}

// HandleDelayTestGitHub 测试 GitHub 延迟
func HandleDelayTestGitHub(w http.ResponseWriter, r *http.Request) {
	handleDelayTestCommon(w, r, "https://github.com")
}

// HandleDelayTestBaidu 测试 Baidu 延迟
func HandleDelayTestBaidu(w http.ResponseWriter, r *http.Request) {
	handleDelayTestCommon(w, r, "https://www.baidu.com")
}

// HandleDelayTestBilibili 测试 Bilibili 延迟
func HandleDelayTestBilibili(w http.ResponseWriter, r *http.Request) {
	handleDelayTestCommon(w, r, "https://www.bilibili.com")
}

// HandleDelayTestCustom 测试自定义 URL 延迟
func HandleDelayTestCustom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	customURL := r.URL.Query().Get("url")
	if customURL == "" {
		httpx.WriteJSONError(w, http.StatusBadRequest, "缺少 url 参数")
		return
	}
	// 简单校验是否以 http:// 或 https:// 开头
	if !strings.HasPrefix(customURL, "http://") && !strings.HasPrefix(customURL, "https://") {
		httpx.WriteJSONError(w, http.StatusBadRequest, "url 必须以 http:// 或 https:// 开头")
		return
	}
	timeoutMs := 5000
	if t := r.URL.Query().Get("timeout"); t != "" {
		if val, err := strconv.Atoi(t); err == nil && val > 0 {
			timeoutMs = val
		}
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	delay, err := testDelayThroughProxy(customURL, timeout)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"delay": nil, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"delay": delay})
}

// 公共处理函数
func handleDelayTestCommon(w http.ResponseWriter, r *http.Request, targetURL string) {
	if r.Method != http.MethodGet {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}
	// 读取超时参数，默认5000ms
	timeoutMs := 5000
	if t := r.URL.Query().Get("timeout"); t != "" {
		if val, err := strconv.Atoi(t); err == nil && val > 0 {
			timeoutMs = val
		}
	}
	timeout := time.Duration(timeoutMs) * time.Millisecond
	delay, err := testDelayThroughProxy(targetURL, timeout)
	if err != nil {
		// 超时或错误，返回 delay=null 或 -1，前端显示超时
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"delay": nil, "error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"delay": delay})
}
