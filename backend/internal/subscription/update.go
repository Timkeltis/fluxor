package subscription

import (
	"fluxor/internal/config"
	"fluxor/internal/subscription/download"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// updateSubscriptionInSwitchMode 切换模式下的订阅更新逻辑，返回 needsReload 表示是否需要重载内核
func updateSubscriptionInSwitchMode(cfg *config.SubscribeConfig, subName string) (needsReload bool, err error) {
	var idx int = -1
	for i, s := range cfg.Subscriptions {
		if s.Name == subName {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, fmt.Errorf("订阅 %s 不存在", subName)
	}

	proxiesDir := filepath.Join(config.CoreWorkDir, "proxies")
	targetFile := filepath.Join(proxiesDir, subName+".yaml")

	// 强制删除已有文件（确保重新下载）
	if err := os.Remove(targetFile); err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("删除旧文件失败: %w", err)
	}

	log.Printf("[UPDATE] 开始下载订阅 %s，目标文件: %s", subName, targetFile)

	updatedAt, subInfo, err := download.DownloadSubscriptionFile(cfg.Subscriptions[idx], idx, targetFile)
	if err != nil {
		log.Printf("[UPDATE] 下载订阅 %s 失败: %v", subName, err)
		return false, fmt.Errorf("下载失败: %w", err)
	}
	cfg.Subscriptions[idx].UpdatedAt = updatedAt
	cfg.Subscriptions[idx].SubscriptionInfo = subInfo
	log.Printf("[UPDATE] 元数据已更新: updatedAt=%s", updatedAt)

	// 打补丁
	log.Printf("[UPDATE] 开始打补丁: %s", targetFile)
	if err := patchSubscriptionFile(targetFile, *cfg); err != nil {
		log.Printf("[UPDATE] 打补丁失败: %v", err)
		return false, fmt.Errorf("打补丁失败: %w", err)
	}
	log.Printf("[UPDATE] 补丁完成")

	// 如果该订阅是当前激活的订阅，则复制到 configTarget，并标记需要重载
	if cfg.Mode == "switch" && cfg.ActiveSubscription == subName {
		log.Printf("[UPDATE] 当前订阅为激活订阅，开始复制配置文件到 %s", config.ConfigTarget)
		if err := copyFile(targetFile, config.ConfigTarget); err != nil {
			log.Printf("[UPDATE] 复制失败: %v", err)
			return false, fmt.Errorf("复制配置文件失败: %w", err)
		}
		log.Printf("[UPDATE] 复制完成")
		return true, nil // 需要重载
	}

	log.Printf("[UPDATE] 当前订阅非激活订阅，跳过复制和重载")
	return false, nil
}
