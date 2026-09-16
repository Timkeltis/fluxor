package subscription

import (
	"encoding/json"
	"fluxor/internal/config"
	"fluxor/internal/configgen"
	"fluxor/internal/core"
	"fluxor/internal/httpx"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// HandleGenerateConfig 处理 POST /subscribe/generate：
// 保存配置、按当前模式生成或复制 config.yaml，并热重载内核。
func HandleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var cfg config.SubscribeConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	if cfg.MetaBackendURL != "" && !httpx.BackendURLRegex.MatchString(cfg.MetaBackendURL) {
		httpx.WriteJSONError(w, http.StatusBadRequest, "外部面板后端地址格式不正确")
		return
	}

	// 物理清理标记删除的配置文件，使用 filepath.Base 防范路径穿越
	if len(cfg.DeletePhysical) > 0 {
		for _, name := range cfg.DeletePhysical {
			cleanName := filepath.Base(name)
			if cleanName == "." || cleanName == "/" || cleanName == "\\" {
				continue
			}
			targetFile := filepath.Join(config.CoreWorkDir, "proxies", cleanName+".yaml")
			if _, err := os.Stat(targetFile); err == nil {
				if err := os.Remove(targetFile); err != nil {
					log.Printf("[DELETE] 物理删除配置文件失败 %s: %v", targetFile, err)
				} else {
					log.Printf("[DELETE] 成功物理删除配置文件: %s", targetFile)
				}
			}
		}
	}
	cfg.DeletePhysical = nil // 清空临时字段避免持久化

	// 切换模式
	if cfg.Mode == "switch" {
		// 如果订阅列表为空，生成基础配置，清除选中状态，保存并重载
		if len(cfg.Subscriptions) == 0 {
			// 生成基础配置文件
			if err := configgen.GenerateBaseConfig(cfg); err != nil {
				httpx.WriteJSONError(w, http.StatusInternalServerError, "生成基础配置失败: "+err.Error())
				return
			}
			// 清除选中的订阅
			cfg.ActiveSubscription = ""
			// 保存配置到全局并持久化
			config.Mu.Lock()
			config.Current = cfg
			config.Mu.Unlock()
			if err := config.SaveSubscribeConfig(); err != nil {
				log.Printf("保存订阅配置失败: %v", err)
			}
			// 重置定时器（无订阅时需停止所有定时器）
			StopAllTimers()
			StartAllTimers() // 会检查模式，切换模式且无订阅时会跳过启动
			// 重载内核
			if err := core.ReloadCore(); err != nil {
				httpx.RespondJSON(w, http.StatusOK, map[string]string{
					"status":  "warning",
					"message": "基础配置已生成，但重载内核失败: " + err.Error(),
				})
				return
			}
			httpx.RespondJSON(w, http.StatusOK, map[string]string{
				"status":  "ok",
				"message": "已清除订阅，切换到基础配置",
			})
			return
		}

		// 有订阅时，确保所有订阅文件已下载
		if err := ensureSubscriptionFiles(&cfg); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "下载订阅文件失败: "+err.Error())
			return
		}
		// 检查是否选中了订阅
		if cfg.ActiveSubscription == "" {
			httpx.WriteJSONError(w, http.StatusBadRequest, "切换模式下请先选择一个订阅")
			return
		}
		// 构建源文件路径
		srcFile := filepath.Join(config.CoreWorkDir, "proxies", cfg.ActiveSubscription+".yaml")
		if _, err := os.Stat(srcFile); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "选中的订阅文件不存在: "+err.Error())
			return
		}
		// 复制文件到 configTarget
		if err := copyFile(srcFile, config.ConfigTarget); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "复制配置文件失败: "+err.Error())
			return
		}
		// 保存配置到 subscribe.json
		config.Mu.Lock()
		config.Current = cfg
		config.Mu.Unlock()
		if err := config.SaveSubscribeConfig(); err != nil {
			log.Printf("保存订阅配置失败: %v", err)
		}
		// 重置定时器
		StopAllTimers()
		StartAllTimers()
		// 重载内核
		if err := core.ReloadCore(); err != nil {
			httpx.RespondJSON(w, http.StatusOK, map[string]string{
				"status":  "warning",
				"message": "配置文件已复制，但重载内核失败: " + err.Error(),
			})
			return
		}
		httpx.RespondJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "已切换到订阅 " + cfg.ActiveSubscription + " 的配置",
		})
		return
	}

	// ---------- 融合模式（原有逻辑） ----------
	// 删除旧配置
	if _, err := os.Stat(config.ConfigTarget); err == nil {
		if err := os.Remove(config.ConfigTarget); err != nil {
			httpx.WriteJSONError(w, http.StatusInternalServerError, "删除旧配置文件失败: "+err.Error())
			return
		}
	}

	config.Mu.Lock()
	config.Current = cfg
	config.Mu.Unlock()
	if err := config.SaveSubscribeConfig(); err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	// 重置定时器
	StopAllTimers()
	StartAllTimers()

	if err := configgen.GenerateConfig(cfg); err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "生成配置文件失败: "+err.Error())
		return
	}

	if cfg.MetaBackendURL != "" {
		if err := modifyMetaConfig(cfg.MetaBackendURL); err != nil {
			log.Printf("[WARN] 修改 MetaCubeXD 后端地址失败: %v", err)
		}
	}

	if err := core.ReloadCore(); err != nil {
		httpx.RespondJSON(w, http.StatusOK, map[string]string{
			"status":  "warning",
			"message": "配置文件已生成，但重载内核失败: " + err.Error(),
		})
		return
	}

	updateAllSubscriptionsMetadata(&cfg)
	// 将更新后的 cfg 保存到全局并持久化
	config.Mu.Lock()
	config.Current = cfg
	config.Mu.Unlock()
	if err := config.SaveSubscribeConfig(); err != nil {
		log.Printf("保存订阅配置失败: %v", err)
	}

	httpx.RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "配置文件已生成并成功重载内核",
	})
}
