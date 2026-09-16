package dashapi

import (
	"fluxor/internal/core"
	"fluxor/internal/httpx"
	"io"
	"net/http"
)

// HandleVersion 返回内核版本信息（代理 /version）
func HandleVersion(w http.ResponseWriter, _ *http.Request) {
	resp, err := core.CoreRequest("GET", "/version", nil)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadGateway, "无法获取内核版本: "+err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

// handleTraffic 返回实时流量信息（代理 /traffic）
func handleTraffic(w http.ResponseWriter, _ *http.Request) {
	resp, err := core.CoreRequest("GET", "/traffic", nil)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadGateway, "无法获取流量信息: "+err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

// handleMemory 返回内存使用信息（代理 /memory）
func handleMemory(w http.ResponseWriter, _ *http.Request) {
	resp, err := core.CoreRequest("GET", "/memory", nil)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadGateway, "无法获取内存信息: "+err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}

// handleConnections 返回连接信息（代理 /connections）
func handleConnections(w http.ResponseWriter, _ *http.Request) {
	resp, err := core.CoreRequest("GET", "/connections", nil)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusBadGateway, "无法获取连接信息: "+err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	io.Copy(w, resp.Body)
}
