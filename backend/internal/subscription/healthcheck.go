package subscription

import (
	"fluxor/internal/config"
	"fluxor/internal/dashapi"
	"log"
	"sync"
	"time"
)

var (
	healthCheckTicker *time.Ticker
	healthCheckStop   chan struct{}
	lastHealthCheck   map[string]time.Time
	healthCheckMu     sync.RWMutex
)

// startHealthCheckTimer 启动健康检查定时器（仅在 switch 模式下生效）
func startHealthCheckTimer() {
	if healthCheckTicker != nil {
		return
	}
	healthCheckStop = make(chan struct{})
	healthCheckMu.Lock()
	lastHealthCheck = make(map[string]time.Time)
	// 将当前激活订阅的 lastHealthCheck 设为当前时间，避免启动后立即测速
	config.Mu.RLock()
	if config.Current.Mode == "switch" && config.Current.ActiveSubscription != "" {
		lastHealthCheck[config.Current.ActiveSubscription] = time.Now()
	}
	config.Mu.RUnlock()
	healthCheckMu.Unlock()

	healthCheckTicker = time.NewTicker(10 * time.Second) // 每10秒检查一次
	go func() {
		for {
			select {
			case <-healthCheckTicker.C:
				performHealthChecks()
			case <-healthCheckStop:
				return
			}
		}
	}()
}

// stopHealthCheckTimer 停止健康检查定时器
func stopHealthCheckTimer() {
	if healthCheckTicker != nil {
		healthCheckTicker.Stop()
		healthCheckTicker = nil
	}
	if healthCheckStop != nil {
		close(healthCheckStop)
		healthCheckStop = nil
	}
	healthCheckMu.Lock()
	lastHealthCheck = nil
	healthCheckMu.Unlock()
}

// performHealthChecks 执行健康检查（仅在 switch 模式下，对当前激活订阅测速）
func performHealthChecks() {
	config.Mu.RLock()
	cfg := config.Current
	config.Mu.RUnlock()

	if cfg.Mode != "switch" || len(cfg.Subscriptions) == 0 || cfg.ActiveSubscription == "" {
		return
	}

	var activeSub *config.Subscription
	for i := range cfg.Subscriptions {
		if cfg.Subscriptions[i].Name == cfg.ActiveSubscription {
			activeSub = &cfg.Subscriptions[i]
			break
		}
	}
	if activeSub == nil {
		return
	}

	interval := activeSub.HealthInterval
	if interval <= 0 {
		interval = 600
	}

	now := time.Now()
	healthCheckMu.RLock()
	last, ok := lastHealthCheck[activeSub.Name]
	healthCheckMu.RUnlock()
	if ok && now.Sub(last) < time.Duration(interval)*time.Second {
		return
	}

	groups, err := dashapi.GetAllProxyGroups()
	if err != nil {
		log.Printf("[HealthCheck] 获取策略组失败: %v", err)
		return
	}
	if len(groups) == 0 {
		return
	}

	// 并发测速（限制并发数5）
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)
	for _, g := range groups {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			dashapi.TestGroupDelay(name)
		}(g)
	}
	wg.Wait()

	healthCheckMu.Lock()
	lastHealthCheck[activeSub.Name] = time.Now()
	healthCheckMu.Unlock()
}
