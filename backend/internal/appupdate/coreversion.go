package appupdate

import (
	"context"
	"encoding/json"
	"fluxor/internal/config"
	"fluxor/internal/httpx"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// 内核远程版本缓存
	latestCoreVersionCache     string
	latestCoreVersionCacheTime time.Time
	coreCacheMutex             sync.RWMutex
	coreCacheTTL               = 10 * time.Minute
)

// getLatestCoreVersion 获取 Mihomo 远程最新稳定版本（从 GitHub releases/latest）
func getLatestCoreVersion() (string, error) {
	coreCacheMutex.RLock()
	if latestCoreVersionCache != "" && time.Since(latestCoreVersionCacheTime) < coreCacheTTL {
		coreCacheMutex.RUnlock()
		return latestCoreVersionCache, nil
	}
	coreCacheMutex.RUnlock()

	url := "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest"
	resp, err := ghAPIClient.Get(url) // 带 15s 超时，避免无响应时挂死
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API 返回状态码 %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	version := strings.TrimPrefix(result.TagName, "v") // 去掉前缀 'v'

	coreCacheMutex.Lock()
	latestCoreVersionCache = version
	latestCoreVersionCacheTime = time.Now()
	coreCacheMutex.Unlock()
	return version, nil
}

// getLocalCoreVersion 通过 Unix Socket 获取本地内核版本
func getLocalCoreVersion() (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", config.CoreSocket)
			},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("http://localhost/version")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("core 版本请求失败: %d", resp.StatusCode)
	}
	var data struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	return strings.TrimPrefix(data.Version, "v"), nil
}

// HandleCoreCheckUpdate 检查内核更新
func HandleCoreCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. 获取本地版本
	localVer, err := getLocalCoreVersion()
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "获取本地内核版本失败: "+err.Error())
		return
	}

	// 2. 判断是否为 Alpha 版本
	if strings.Contains(strings.ToLower(localVer), "alpha") {
		// 解析本地哈希（格式：alpha-978d25a）
		localHash := ""
		parts := strings.SplitN(localVer, "-", 2)
		if len(parts) == 2 {
			localHash = parts[1]
		}

		// 获取远程 Alpha 哈希
		remoteHash, err := getLatestAlphaCoreHash()
		if err != nil {
			// 获取失败时返回错误，但保持用户体验，可返回无更新
			httpx.WriteJSONError(w, http.StatusServiceUnavailable, "获取 Alpha 版本信息失败: "+err.Error())
			return
		}

		hasUpdate := localHash != remoteHash
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hasUpdate": hasUpdate,
			"latest":    "alpha-" + remoteHash,
			"local":     localVer,
		})
		return
	}

	// 3. 稳定版逻辑（原有）
	// 若本地版本包含 "alpha"，已经返回，以下为稳定版处理
	remoteVer, err := getLatestCoreVersion()
	if err != nil {
		httpx.WriteJSONError(w, http.StatusServiceUnavailable, "获取远程版本失败: "+err.Error())
		return
	}
	hasUpdate := compareVersions(remoteVer, localVer) > 0
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hasUpdate": hasUpdate,
		"latest":    remoteVer,
		"local":     localVer,
	})
}
