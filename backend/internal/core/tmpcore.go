package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fluxor/internal/config"
	"fluxor/internal/configcheck"
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
	"strings"
	"sync"
	"syscall"
	"time"
)

// errInvalidSubscription 表示订阅内容本身无法被内核解析（而非端口冲突、进程崩溃等
// 可重试的瞬时故障）。这类失败重试不会改变结果，应直接返回。
var errInvalidSubscription = errors.New("订阅内容无法被内核解析")

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

		content, buildErr := buildTempCoreConfig(port, sub, targetFile)
		if buildErr != nil {
			lastErr = buildErr
			continue
		}

		if err = os.WriteFile(tmpConfig, []byte(content), 0644); err != nil {
			lastErr = fmt.Errorf("写入临时配置失败: %w", err)
			continue
		}

		cmd := exec.Command(config.CoreBin, "-f", tmpConfig, "-d", config.CoreWorkDir)
		// mihomo 的日志（含 provider 加载失败原因）写入 stdout 而非 stderr，
		// 因此必须捕获 stdout，否则失败原因会全部丢失。
		output := newSyncBuffer()
		cmd.Stdout = output
		cmd.Stderr = output

		if err = cmd.Start(); err != nil {
			lastErr = fmt.Errorf("启动临时内核失败: %w, 输出: %s", err, output.String())
			os.Remove(tmpConfig)
			continue
		}

		// 保存 PID
		_ = os.WriteFile(tmpPidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)

		// 监测是否因端口冲突在启动瞬间退出
		time.Sleep(100 * time.Millisecond)
		if cmd.Process != nil && cmd.Process.Signal(syscall.Signal(0)) != nil {
			lastErr = fmt.Errorf("临时内核启动后立即退出，可能端口冲突，输出: %s", output.String())
			os.Remove(tmpConfig)
			os.Remove(tmpPidFile)
			continue
		}

		// 成功运行，开始处理下载
		updatedAt, subInfo, err = runDownloadProcess(cmd, targetFile, port, sub.Name, tmpConfig, tmpPidFile, output)
		if err == nil {
			return updatedAt, subInfo, nil
		}
		// 订阅内容本身无法被内核解析属于确定性失败，重试不会改变结果
		if errors.Is(err, errInvalidSubscription) {
			return "", nil, err
		}
		lastErr = err
	}

	return "", nil, fmt.Errorf("下载订阅失败，已尝试3次，最后错误: %w", lastErr)
}

// buildTempCoreConfig 生成临时内核的最小配置（仅含一个 http provider）。
//
// 用 YAML 文档而非字符串拼接：订阅名作为 provider 的键，可能含 `:` 等 YAML
// 特殊字符（如「机场: 香港」），插值会产出内核无法解析的配置。经文档写入
// 可正确转义。
func buildTempCoreConfig(port int, sub config.Subscription, targetFile string) (string, error) {
	doc := configcheck.NewDoc()
	if err := doc.Set("mixed-port", 0); err != nil {
		return "", err
	}
	if err := doc.Set("log-level", "error"); err != nil {
		return "", err
	}
	if err := doc.Set("external-controller", fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		return "", err
	}
	if err := doc.Set("proxy-providers", tempCoreProviders(sub, targetFile)); err != nil {
		return "", err
	}

	out, err := doc.Bytes()
	if err != nil {
		return "", fmt.Errorf("序列化临时内核配置失败: %w", err)
	}
	return string(out), nil
}

// tempCoreProviders 构建形如 {<订阅名>: {type, url, path}} 的 provider 映射。
func tempCoreProviders(sub config.Subscription, targetFile string) map[string]any {
	return map[string]any{
		sub.Name: map[string]any{
			"type": "http",
			"url":  sub.URL,
			"path": targetFile,
		},
	}
}

func runDownloadProcess(cmd *exec.Cmd, targetFile string, port int, subName string, tmpConfig string, tmpPidFile string, output *syncBuffer) (updatedAt string, subInfo map[string]interface{}, err error) {
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

	// 轮询等待目标文件生成，同时检查进程存活。
	//
	// provider 加载失败时内核不会退出、也不会写文件，只在日志中留下一行 error
	// （见 providerLoadError），若仅等待文件生成就会一直卡到超时，因此一旦发现
	// 内核已明确报错便立即终止，不再空等。
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	fileGenerated := false
	for !fileGenerated {
		select {
		case <-timeout:
			if reason := providerLoadError(output.String(), subName); reason != "" {
				return "", nil, fmt.Errorf("%w: 订阅内容无法被内核解析: %s", errInvalidSubscription, reason)
			}
			return "", nil, fmt.Errorf("下载超时（60秒），文件未生成")
		case <-ticker.C:
			// 检查进程是否存活
			if cmd.Process == nil || cmd.Process.Signal(syscall.Signal(0)) != nil {
				return "", nil, fmt.Errorf("临时内核进程意外退出")
			}
			info, statErr := os.Stat(targetFile)
			if statErr == nil && info.Size() > 0 {
				// 内核按 provider 契约写回的是「原始响应」，未必是主配置契约下的
				// Clash YAML（例如 Base64 编码的 URI 列表会被原样落盘）。后续
				// patchSubscriptionFile 会在这份内容上拼接 YAML 键，因此必须先校验，
				// 否则会产出内核无法加载的 config.yaml。
				if content, readErr := os.ReadFile(targetFile); readErr == nil {
					if validErr := configcheck.ValidateClashConfig(content); validErr != nil {
						log.Printf("[DOWNLOAD] 内核产出的订阅文件无效，终止: %s", validErr)
						return "", nil, fmt.Errorf("%w: 内核可读取该订阅但产出内容不是 Clash 配置: %s",
							errInvalidSubscription, validErr)
					}
				}
				log.Printf("[DOWNLOAD] 文件 %s 已生成，大小 %d 字节", targetFile, info.Size())
				fileGenerated = true
				break
			}
			// 内核已明确判定 provider 加载失败，无需继续等待
			if reason := providerLoadError(output.String(), subName); reason != "" {
				log.Printf("[DOWNLOAD] 内核加载 provider %s 失败，提前终止: %s", subName, reason)
				return "", nil, fmt.Errorf("%w: 该订阅链接不是 Clash 配置，内核解析失败: %s", errInvalidSubscription, reason)
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

// providerLoadError 从临时内核输出中提取指定 provider 的加载失败原因。
//
// 内核在 provider 初始化失败时会输出形如：
//
//	level=error msg="initial proxy provider <name> error: <原因>"
//
// 返回空串表示内核尚未报告该 provider 的加载错误。
func providerLoadError(output string, subName string) string {
	marker := "initial proxy provider " + subName + " error:"
	for _, line := range strings.Split(output, "\n") {
		idx := strings.Index(line, marker)
		if idx < 0 {
			continue
		}
		reason := strings.TrimSpace(line[idx+len(marker):])
		// 去掉 msg 字段的收尾引号（含 \" 转义）
		reason = strings.TrimSuffix(reason, `"`)
		// 内核把多行错误压成 `标题:\n 实际原因` 的转义形式，
		// 还原后只保留最有信息量的那段，避免展示无意义的标题。
		reason = strings.ReplaceAll(reason, `\n`, "\n")
		reason = strings.ReplaceAll(reason, `\"`, `"`)
		return condenseReason(reason)
	}
	return ""
}

// condenseReason 把内核错误压成单行：yaml.v3 的错误首行只是
// 「yaml: unmarshal errors:」这样的标题，真正的原因在其后。
func condenseReason(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return strings.TrimSpace(s)
}

// syncBuffer 是并发安全的输出缓冲：由内核子进程写入，父进程并发读取。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{} }

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
