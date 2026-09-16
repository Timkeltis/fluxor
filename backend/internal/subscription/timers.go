package subscription

import (
	"context"
	"fluxor/internal/config"
	"fluxor/internal/core"
	"log"
	"sync"
	"time"
)

var (
	timerCancel map[string]context.CancelFunc
	timerMu     sync.RWMutex
)

func init() {
	timerCancel = make(map[string]context.CancelFunc)
}

// startSubscriptionTimer 为指定订阅启动定时更新（仅切换模式）
func startSubscriptionTimer(cfg *config.SubscribeConfig, idx int) {
	if cfg.Mode != "switch" {
		return
	}
	sub := cfg.Subscriptions[idx]
	if sub.UpdateInterval <= 0 {
		return
	}

	timerMu.Lock()
	defer timerMu.Unlock()

	// 取消旧定时器
	if cancel, ok := timerCancel[sub.Name]; ok {
		cancel()
		delete(timerCancel, sub.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	timerCancel[sub.Name] = cancel

	go func(name string, interval int) {
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 执行更新（需要获取锁）
				var needsReload bool
				var err error
				config.Mu.Lock()
				// 检查模式是否仍然为 switch 且订阅仍存在
				if config.Current.Mode != "switch" {
					config.Mu.Unlock()
					return
				}
				// 查找订阅索引
				var idxFound int = -1
				for i, s := range config.Current.Subscriptions {
					if s.Name == name {
						idxFound = i
						break
					}
				}
				if idxFound == -1 {
					config.Mu.Unlock()
					return
				}
				needsReload, err = updateSubscriptionInSwitchMode(&config.Current, name)
				config.Mu.Unlock()
				// 执行更新
				if err != nil {
					log.Printf("定时更新订阅 %s 失败: %v", name, err)
				} else {
					if err := config.SaveSubscribeConfig(); err != nil {
						log.Printf("保存订阅配置失败: %v", err)
					}
					if needsReload {
						if err := core.ReloadCore(); err != nil {
							log.Printf("定时更新后重载内核失败: %v", err)
						}
					}
				}
			}
		}
	}(sub.Name, sub.UpdateInterval)
}

// StartAllTimers 按当前模式启动所有定时任务：
// switch 模式下为每个订阅启动更新定时器，并额外启动健康检查定时器。
func StartAllTimers() {
	config.Mu.RLock()
	defer config.Mu.RUnlock()
	if config.Current.Mode != "switch" {
		return
	}
	for i := range config.Current.Subscriptions {
		startSubscriptionTimer(&config.Current, i)
	}
	startHealthCheckTimer()
}

// StopAllTimers 停止所有定时器
func StopAllTimers() {
	timerMu.Lock()
	defer timerMu.Unlock()
	for name, cancel := range timerCancel {
		cancel()
		delete(timerCancel, name)
	}
	stopHealthCheckTimer()
}
