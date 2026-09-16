package core

import (
	"bytes"
	"encoding/json"
	"fluxor/internal/config"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// DownloadWithTempCore 使用临时内核下载单个订阅的节点文件，并返回元数据（updatedAt 和 subscriptionInfo）
func DownloadWithTempCore(sub config.Subscription, index int, targetFile string) (updatedAt string, subInfo map[string]interface{}, err error) {
	var lastErr error
	var port int
	var listener net.Listener

	tmpDir := filepath.Dir(config.CorePidFile)
	tmpConfig := filepath.Join(tmpDir, fmt.Sprintf("tmp%d.yaml", index))
	tmpPidFile := filepath.Join(tmpDir, fmt.Sprintf("tmp%d.pid", index))

	for attempt := 0; attempt < 3; attempt++ {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			lastErr = fmt.Errorf("分配端口失败: %w", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()

		content := fmt.Sprintf(`mixed-port: 0
log-level: silent
external-controller: '127.0.0.1:%d'
proxy-providers:
  %s:
    type: http
    url: "%s"
    path: "%s"
`, port, sub.Name, sub.URL, targetFile)

		if err = os.WriteFile(tmpConfig, []byte(content), 0644); err != nil {
			lastErr = fmt.Errorf("写入临时配置失败: %w", err)
			continue
		}

		cmd := exec.Command(config.CoreBin, "-f", tmpConfig, "-d", config.CoreWorkDir)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err = cmd.Start(); err != nil {
			lastErr = fmt.Errorf("启动临时内核失败: %w, stderr: %s", err, stderr.String())
			os.Remove(tmpConfig)
			continue
		}

		// 保存 PID
		_ = os.WriteFile(tmpPidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

		// 监测是否因端口冲突在启动瞬间退出
		time.Sleep(100 * time.Millisecond)
		if cmd.Process != nil && cmd.Process.Signal(syscall.Signal(0)) != nil {
			lastErr = fmt.Errorf("临时内核启动后立即退出，可能端口冲突，stderr: %s", stderr.String())
			os.Remove(tmpConfig)
			os.Remove(tmpPidFile)
			continue
		}

		// 成功运行，开始处理下载
		updatedAt, subInfo, err = runDownloadProcess(cmd, targetFile, port, sub.Name, tmpConfig, tmpPidFile)
		if err == nil {
			return updatedAt, subInfo, nil
		}
		lastErr = err
	}

	return "", nil, fmt.Errorf("下载订阅失败，已尝试3次，最后错误: %w", lastErr)
}

func runDownloadProcess(cmd *exec.Cmd, targetFile string, port int, subName string, tmpConfig string, tmpPidFile string) (updatedAt string, subInfo map[string]interface{}, err error) {
	defer func() {
		os.Remove(tmpConfig)
		os.Remove(tmpPidFile)
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			done := make(chan error, 1)
			go func() {
				done <- cmd.Wait()
			}()
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
	}()

	// 轮询等待目标文件生成（60秒），同时检查进程存活
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fileGenerated := false
	for !fileGenerated {
		select {
		case <-timeout:
			return "", nil, fmt.Errorf("下载超时（60秒），文件未生成")
		case <-ticker.C:
			// 检查进程是否存活
			if cmd.Process == nil || cmd.Process.Signal(syscall.Signal(0)) != nil {
				return "", nil, fmt.Errorf("临时内核进程意外退出")
			}
			info, err := os.Stat(targetFile)
			if err == nil && info.Size() > 0 {
				log.Printf("[DOWNLOAD] 文件 %s 已生成，大小 %d 字节", targetFile, info.Size())
				fileGenerated = true
			}
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	urlPath := fmt.Sprintf("http://127.0.0.1:%d/providers/proxies/%s", port, url.QueryEscape(subName))
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < 30; attempt++ {
		resp, lastErr = client.Get(urlPath)
		if lastErr == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return "", nil, fmt.Errorf("获取订阅元数据失败: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("获取元数据返回非200状态: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", nil, fmt.Errorf("解析JSON失败: %w", err)
	}

	updatedAtVal, _ := data["updatedAt"].(string)
	subInfoVal, _ := data["subscriptionInfo"].(map[string]interface{})

	log.Printf("[DOWNLOAD] 成功获取元数据: updatedAt=%s, subInfo=%v", updatedAtVal, subInfoVal)
	return updatedAtVal, subInfoVal, nil
}
