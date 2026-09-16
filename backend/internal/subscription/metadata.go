package subscription

import (
	"encoding/json"
	"fluxor/internal/config"
	"fluxor/internal/core"
	"fluxor/internal/subscription/download"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// fetchSubscriptionMetadataFromCore 从主内核获取订阅元数据
func fetchSubscriptionMetadataFromCore(subName string) (updatedAt string, subInfo map[string]interface{}, err error) {
	encoded := url.QueryEscape(subName)
	resp, err := core.CoreRequest("GET", "/providers/proxies/"+encoded, nil)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("状态码: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", nil, err
	}
	updatedAtVal, _ := data["updatedAt"].(string)
	subInfoVal, _ := data["subscriptionInfo"].(map[string]interface{})
	// 规范化键为小写
	subInfoVal = download.NormalizeMapKeys(subInfoVal)
	return updatedAtVal, subInfoVal, nil
}

// updateAllSubscriptionsMetadata 更新所有订阅的元数据（仅用于融合模式）
func updateAllSubscriptionsMetadata(cfg *config.SubscribeConfig) {
	// 1. 备份当前全局配置中的旧元数据（用于失败时保留）
	config.Mu.RLock()
	oldSubs := make(map[string]config.Subscription)
	for _, s := range config.Current.Subscriptions {
		oldSubs[s.Name] = s
	}
	config.Mu.RUnlock()

	// 2. 遍历每个订阅，尝试获取元数据
	for i := range cfg.Subscriptions {
		name := cfg.Subscriptions[i].Name
		var updatedAt string
		var subInfo map[string]interface{}
		var err error

		// 3. 重试机制：最多尝试 3 次，每次间隔 500ms
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(500 * time.Millisecond)
			}
			updatedAt, subInfo, err = fetchSubscriptionMetadataFromCore(name)
			if err == nil {
				break
			}
			log.Printf("获取订阅 %s 元数据失败 (尝试 %d/%d): %v", name, attempt+1, 3, err)
		}

		if err != nil {
			// 获取失败：尝试保留旧数据
			if old, ok := oldSubs[name]; ok {
				cfg.Subscriptions[i].UpdatedAt = old.UpdatedAt
				cfg.Subscriptions[i].SubscriptionInfo = old.SubscriptionInfo
				log.Printf("保留订阅 %s 的旧元数据（获取失败）", name)
			} else {
				cfg.Subscriptions[i].UpdatedAt = ""
				cfg.Subscriptions[i].SubscriptionInfo = nil
				log.Printf("订阅 %s 无历史元数据，保留为空", name)
			}
			continue
		}

		// 获取成功，更新
		cfg.Subscriptions[i].UpdatedAt = updatedAt
		cfg.Subscriptions[i].SubscriptionInfo = subInfo
		log.Printf("更新订阅 %s 元数据成功", name)
	}
}
