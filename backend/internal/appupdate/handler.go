package appupdate

import (
	"encoding/json"
	"fluxor/internal/httpx"
	"net/http"
)

// HandleCheckUpdate 检查 Fluxor 自身是否有新版本
func HandleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	current := r.URL.Query().Get("current")
	if current == "" {
		httpx.WriteJSONError(w, http.StatusBadRequest, "missing current version")
		return
	}
	current = stripVersionSuffix(current)

	// 尝试获取完整 Release 信息（含更新日志）
	rel, err := getLatestReleaseInfo()
	if err != nil {
		// 降级方案：仅获取版本号（不返回 releaseNotes）
		latest, err2 := getLatestVersion()
		if err2 != nil {
			httpx.WriteJSONError(w, http.StatusServiceUnavailable, "failed to check update: "+err2.Error())
			return
		}
		hasUpdate := compareVersions(latest, current) > 0
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hasUpdate": hasUpdate,
			"latest":    latest,
			"current":   current,
			// 无 releaseNotes 字段
		})
		return
	}

	hasUpdate := compareVersions(rel.TagName, current) > 0
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"hasUpdate":    hasUpdate,
		"latest":       rel.TagName,
		"current":      current,
		"releaseNotes": rel.Body,
	})
}
