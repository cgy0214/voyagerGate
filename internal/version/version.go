// Package version 统一管理版本号。
package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const Version = "v1.0.1"

const versionURL = "https://raw.githubusercontent.com/cgy0214/voyagerGate/master/version.json"

// 镜像回退：直连 GitHub 失败（如未开代理的国内网络）时依次尝试
var versionMirrorURLs = []string{
	"https://ghproxy.net/https://raw.githubusercontent.com/cgy0214/voyagerGate/master/version.json",
	"https://gh-proxy.com/https://raw.githubusercontent.com/cgy0214/voyagerGate/master/version.json",
}

// httpClient 返回带系统代理与超时的 HTTP 客户端。
// Go 默认不读 Windows 系统代理，这里显式探测注册表设置，
// 保证系统代理用户无需额外配置即可访问 GitHub。
func httpClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if pu := proxyFromSystem(); pu != nil {
				return pu, nil
			}
			return http.ProxyFromEnvironment(req)
		},
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// proxyFromSystem 读取 Windows 系统代理设置（仅 windows 生效）
func proxyFromSystem() *url.URL {
	if runtime.GOOS != "windows" {
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()
	enable, _, err := key.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return nil
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil || server == "" {
		return nil
	}
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return nil
	}
	return u
}

// RemoteVersion 远程 version.json 结构
type RemoteVersion struct {
	Version  string `json:"version"`
	Note     string `json:"note,omitempty"`
	Download string `json:"download,omitempty"`
}

func Display() string {
	return Version
}

// CheckUpdateResult 检查更新返回结果
type CheckUpdateResult struct {
	HasUpdate bool   `json:"hasUpdate"`
	Version   string `json:"version"`
	Note      string `json:"note"`
	Download  string `json:"download"`
	Message   string `json:"message"`
}

// CheckUpdate 从 GitHub 拉取 version.json 比较版本号
func CheckUpdate() string {
	result := checkUpdateInternal()
	return result.Message
}

// CheckUpdateDetail 返回详细结果（供前端展示下载按钮）
func CheckUpdateDetail() *CheckUpdateResult {
	return checkUpdateInternal()
}

func checkUpdateInternal() *CheckUpdateResult {
	body, err := fetchVersionJSON()
	if err != nil {
		return &CheckUpdateResult{Message: "检查更新失败: 网络异常"}
	}

	var remote RemoteVersion
	if err := json.Unmarshal(body, &remote); err != nil || remote.Version == "" {
		return &CheckUpdateResult{Message: "检查更新失败: 版本信息格式错误"}
	}

	remote = normalizeVersion(remote)
	local := normalizeVersion(RemoteVersion{Version: Version})

	if remote.Version == local.Version {
		return &CheckUpdateResult{
			Message: "已是最新版本 " + Display() + " ✓",
		}
	}

	download := remote.Download
	if download == "" {
		download = "https://github.com/cgy0214/voyagerGate/releases/latest"
	}

	return &CheckUpdateResult{
		HasUpdate: true,
		Version:   remote.Version,
		Note:      remote.Note,
		Download:  download,
		Message:   fmt.Sprintf("发现新版本 %s（当前 %s）", remote.Version, Display()),
	}
}

// fetchVersionJSON 按主地址 → 镜像列表顺序拉取 version.json，任一成功即返回
func fetchVersionJSON() ([]byte, error) {
	urls := append([]string{versionURL}, versionMirrorURLs...)
	client := httpClient(5 * time.Second)
	var lastErr error
	for _, u := range urls {
		resp, err := client.Get(u)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

func DownloadAndUpdate(downloadURL string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前程序路径失败: %v", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	// 确定下载文件名
	tmpPath := exePath + ".tmp"

	// 下载新版本（走系统代理，避免直连 GitHub 被墙）
	resp, err := httpClient(60 * time.Second).Get(downloadURL)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer func() { out.Close(); os.Remove(tmpPath) }()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}
	out.Close()

	// Windows 下需要关闭当前进程再替换
	if runtime.GOOS == "windows" {
		// 先尝试直接替换（可能失败因为文件正在使用）
		if err := os.Rename(tmpPath, exePath); err != nil {
			// 使用 move.bat 延迟替换：下次启动时替换
			batPath := filepath.Join(filepath.Dir(exePath), "move.bat")
			batContent := fmt.Sprintf("@echo off\ntimeout /t 2 /nobreak >nul\nmove /y \"%s\" \"%s\"\ndel \"%%~f0\"", tmpPath, exePath)
			os.WriteFile(batPath, []byte(batContent), 0644)
			go func() {
				time.Sleep(1 * time.Second)
				exec.Command("cmd", "/c", batPath).Start()
			}()
		}
	} else {
		os.Chmod(tmpPath, 0755)
		if err := os.Rename(tmpPath, exePath); err != nil {
			return fmt.Errorf("替换文件失败: %v", err)
		}
	}

	return nil
}

// normalizeVersion 统一去除版本号前缀 v/V 并 trim
func normalizeVersion(v RemoteVersion) RemoteVersion {
	v.Version = strings.TrimSpace(v.Version)
	v.Version = strings.TrimPrefix(v.Version, "v")
	v.Version = strings.TrimPrefix(v.Version, "V")
	return v
}
