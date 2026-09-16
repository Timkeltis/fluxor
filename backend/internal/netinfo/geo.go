package netinfo

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

var geoCache = struct {
	sync.RWMutex
	data map[string]geoInfo
}{data: make(map[string]geoInfo)}

type geoInfo struct {
	Country string
	Region  string
	Isp     string
	Expire  time.Time
}

// fetchGeoInfo 查询 IP 地理信息，返回 country, region, isp（带缓存和重试）
func fetchGeoInfo(ip string) (string, string, string) {
	if ip == "" {
		return "", "", ""
	}

	// 1. 检查缓存
	geoCache.RLock()
	if cached, ok := geoCache.data[ip]; ok && time.Now().Before(cached.Expire) {
		geoCache.RUnlock()
		return cached.Country, cached.Region, cached.Isp
	}
	geoCache.RUnlock()

	client := &http.Client{Timeout: 5 * time.Second}
	// 使用 ip-api.com 作为主 API（免费版，无 token，但需遵守使用条款）
	// 返回 JSON: {"country":"...", "regionName":"...", "isp":"..."}
	// 注意：ip-api.com 对非商用有限制，但通常可用
	urls := []string{
		"http://ip-api.com/json/" + ip + "?fields=country,regionName,isp",
		"https://api.ip.sb/geoip/" + ip,
	}

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		for _, url := range urls {
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				continue
			}
			var country, region, isp string
			// 尝试解析 JSON
			var data struct {
				Country    string `json:"country"`
				Region     string `json:"region"`
				RegionName string `json:"regionName"`
				Isp        string `json:"isp"`
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err := json.Unmarshal(bodyBytes, &data); err == nil {
				country = data.Country
				if data.RegionName != "" {
					region = data.RegionName
				} else {
					region = data.Region
				}
				isp = data.Isp
			} else {
				// 尝试解析 api.ip.sb 格式
				var data2 struct {
					Country string `json:"country"`
					Region  string `json:"region"`
					Isp     string `json:"isp"`
				}
				if err := json.Unmarshal(bodyBytes, &data2); err == nil {
					country = data2.Country
					region = data2.Region
					isp = data2.Isp
				}
			}
			if country != "" || region != "" || isp != "" {
				// 缓存结果，有效期 10 分钟
				geoCache.Lock()
				geoCache.data[ip] = geoInfo{
					Country: country,
					Region:  region,
					Isp:     isp,
					Expire:  time.Now().Add(10 * time.Minute),
				}
				geoCache.Unlock()
				return country, region, isp
			}
		}
	}
	return "", "", ""
}
