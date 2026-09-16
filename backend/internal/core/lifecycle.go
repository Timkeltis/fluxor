package core

import (
	"bytes"
	"encoding/json"
	"fluxor/internal/config"
	"fluxor/internal/configgen"
	"fluxor/internal/tproxy"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ReloadCore 通过 Unix socket 重载内核配置（热重启）
func ReloadCore() error {
	bodyJSON := fmt.Sprintf(`{"path":"%s"}`, config.ConfigTarget)
	resp, err := CoreRequest(http.MethodPut, "/configs?force=true", strings.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("内核重载请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("内核返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	// 重载成功后，异步更新 nftables TProxy 规则
	go func() {
		time.Sleep(500 * time.Millisecond)
		resp2, err2 := CoreRequest("GET", "/configs", nil)
		if err2 == nil {
			defer resp2.Body.Close()
			var info map[string]interface{}
			if err2 := json.NewDecoder(resp2.Body).Decode(&info); err2 == nil {
				if tp, ok := info["tproxy-port"]; ok {
					if tpf, ok := tp.(float64); ok {
						if tpf > 0 && tproxy.GetTproxyState() {
							tproxy.DisableTProxyRules()
							tproxy.EnableTProxyRules(int(tpf))
						} else {
							tproxy.DisableTProxyRules()
						}
					}
				}
			}
		}
	}()

	return nil
}

// IsCoreRunning 检查内核是否在运行（通过 PID 文件）
func IsCoreRunning() bool {
	data, err := os.ReadFile(config.CorePidFile)
	if err != nil {
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	if pidStr == "" {
		return false
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

// StartCore 启动内核进程
func StartCore() error {
	if IsCoreRunning() {
		return fmt.Errorf("内核已在运行")
	}

	// 确保配置文件存在，若不存在则使用 subscribeConfig 生成；若存在则强制补齐网关属性
	if _, err := os.Stat(config.ConfigTarget); os.IsNotExist(err) {
		if err := configgen.GenerateConfig(config.Current); err != nil {
			if CoreLogger != nil {
				CoreLogger.Printf("[START][ERROR] 生成配置文件失败: %v\n", err)
			}
			return fmt.Errorf("生成配置文件失败: %w", err)
		}
	}

	cmd := exec.Command(config.CoreBin, "-d", config.CoreWorkDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		if CoreLogger != nil {
			CoreLogger.Printf("[START][ERROR] 启动内核失败: %v, stderr: %s\n", err, stderr.String())
		}
		return fmt.Errorf("启动内核失败: %v, stderr: %s", err, stderr.String())
	}

	// 等待 1 秒，检查进程是否存活
	time.Sleep(1 * time.Second)
	err := cmd.Process.Signal(syscall.Signal(0))
	if err != nil {
		stderrContent := stderr.String()
		waitErr := cmd.Wait()
		if waitErr != nil {
			stderrContent += " (Wait err: " + waitErr.Error() + ")"
		}
		if stderrContent == "" {
			stderrContent = "进程已退出，无 stderr 输出"
		}
		if CoreLogger != nil {
			CoreLogger.Printf("[START][ERROR] 内核启动后立即退出: %s\n", stderrContent)
		}
		return fmt.Errorf("内核启动后立即退出: %s", stderrContent)
	}

	pid := cmd.Process.Pid
	os.MkdirAll(filepath.Dir(config.CorePidFile), 0755)
	if err := os.WriteFile(config.CorePidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		if CoreLogger != nil {
			CoreLogger.Printf("[START][ERROR] 写入 PID 文件失败: %v\n", err)
		}
		return fmt.Errorf("写入 PID 文件失败: %v", err)
	}

	// 后台等待进程退出
	go func() {
		cmd.Wait()
		os.Remove(config.CorePidFile)
	}()

	return nil
}

// StopCore 停止内核进程
func StopCore() error {
	tproxy.DisableTProxyRules() // 进程停掉前，立即释放系统 nft 规则
	_ = os.Remove(config.CoreSocket)

	if !IsCoreRunning() {
		return fmt.Errorf("内核未运行，停止操作被忽略")
	}
	data, _ := os.ReadFile(config.CorePidFile)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	process, err := os.FindProcess(pid)
	if err != nil {
		if CoreLogger != nil {
			CoreLogger.Printf("[STOP][ERROR] 查找进程失败: %v\n", err)
		}
		return fmt.Errorf("查找进程失败: %v", err)
	}
	if err := process.Signal(syscall.SIGTERM); err != nil {
		if CoreLogger != nil {
			CoreLogger.Printf("[STOP][ERROR] 停止进程失败: %v\n", err)
		}
		return fmt.Errorf("停止进程失败: %v", err)
	}

	// 轮询检查进程是否退出（最大 5 秒超时，每 100ms 一次）
	killed := false
	for i := 0; i < 50; i++ {
		err := process.Signal(syscall.Signal(0))
		if err != nil {
			killed = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !killed {
		// 超时则发送 SIGKILL 强杀
		process.Signal(syscall.SIGKILL)
		time.Sleep(200 * time.Millisecond)
	}

	os.Remove(config.CorePidFile)
	_ = os.Remove(config.CoreSocket)
	return nil
}
