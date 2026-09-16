package tproxy

import (
	"encoding/json"
	"fluxor/internal/config"
	"fluxor/internal/httpx"
	"log"
	"net/http"
)

// HandleTproxyState 处理开关（保持不变，但 POST 时重新应用规则）
func HandleTproxyState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tproxyMu.RLock()
		enabled := tproxyEnableState
		tproxyMu.RUnlock()
		httpx.RespondJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
	case http.MethodPost:
		var req struct{ Enable bool }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求格式")
			return
		}
		tproxyMu.Lock()
		tproxyEnableState = req.Enable
		tproxyMu.Unlock()

		if req.Enable {
			config.Mu.RLock()
			port := config.Current.TproxyPort
			config.Mu.RUnlock()
			if port > 0 {
				DisableTProxyRules() // 先清理
				if err := EnableTProxyRules(port); err != nil {
					log.Printf("[TProxy] 添加规则失败: %v", err)
				}
			} else {
				log.Printf("[TProxy] 开关开启但端口为0，无法添加规则")
			}
		} else {
			DisableTProxyRules()
		}
		httpx.RespondJSON(w, http.StatusOK, map[string]bool{"enabled": req.Enable})
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// HandleTproxyExceptions 处理例外列表的获取和更新
func HandleTproxyExceptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		exceptionsMu.RLock()
		dst := tproxyDstExceptionsCache
		src := tproxySrcExceptionsCache
		exceptionsMu.RUnlock()
		httpx.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"dst": dst,
			"src": src,
		})
	case http.MethodPost:
		var req struct {
			Dst []string `json:"dst"`
			Src []string `json:"src"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteJSONError(w, http.StatusBadRequest, "无效请求格式")
			return
		}
		// 分别保存
		if err := SaveTproxyDstExceptions(req.Dst); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "保存目的例外失败")
			return
		}
		if err := SaveTproxySrcExceptions(req.Src); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "保存源例外失败")
			return
		}
		// 如果 TProxy 启用则重载
		if tproxyEnableState {
			config.Mu.RLock()
			port := config.Current.TproxyPort
			config.Mu.RUnlock()
			if port > 0 {
				DisableTProxyRules()
				EnableTProxyRules(port)
			}
		}
		httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// HandleTproxyProxyLocal 处理本机代理开关的获取和设置
func HandleTproxyProxyLocal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		exceptionsMu.RLock()
		enabled := tproxyProxyLocal
		exceptionsMu.RUnlock()
		httpx.RespondJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
	case http.MethodPost:
		var req struct{ Enabled bool }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteJSONError(w, http.StatusBadRequest, "无效请求格式")
			return
		}
		if err := SaveTproxyProxyLocal(req.Enabled); err != nil {
			log.Printf("[TProxy] 保存本机代理开关失败: %v", err)
			httpx.WriteJSONError(w, http.StatusInternalServerError, "保存失败")
			return
		}
		// 如果 TProxy 当前启用，立即重新应用规则
		if tproxyEnableState {
			config.Mu.RLock()
			port := config.Current.TproxyPort
			config.Mu.RUnlock()
			if port > 0 {
				DisableTProxyRules()
				if err := EnableTProxyRules(port); err != nil {
					log.Printf("[TProxy] 重新应用规则失败: %v", err)
				}
			}
		}
		httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}
