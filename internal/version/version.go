// Package version 统一管理版本号。
package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// releaseAPIURLs 依次尝试获取最新 Release 信息（含更新说明 body）。
// 优先官方 API，失败则经镜像代理（仍可走系统代理）。
var releaseAPIURLs = []string{
	"https://api.github.com/repos/cgy0214/voyagerGate/releases/latest",
	"https://ghproxy.net/https://api.github.com/repos/cgy0214/voyagerGate/releases/latest",
	"https://gh-proxy.com/https://api.github.com/repos/cgy0214/voyagerGate/releases/latest",
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

// githubRelease GitHub Releases API 返回结构（取用 body / html_url）
type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
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

	// 更新说明优先取 GitHub 最新 Release 正文，失败回退 version.json 的 note
	note := remote.Note
	if rel := fetchReleaseNotes(); rel != "" {
		note = rel
	}

	return &CheckUpdateResult{
		HasUpdate: true,
		Version:   remote.Version,
		Note:      note,
		Download:  download,
		Message:   fmt.Sprintf("发现新版本 %s（当前 %s）", remote.Version, Display()),
	}
}

// fetchReleaseNotes 读取 GitHub 最新 Release 的更新说明正文（releases/latest）。
// 依次尝试官方 API 与各镜像代理；任一成功即返回去空白后的正文，全部失败返回空串。
func fetchReleaseNotes() string {
	client := httpClient(8 * time.Second)
	var lastErr error
	for _, u := range releaseAPIURLs {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		// GitHub API 要求带 User-Agent，否则 403
		req.Header.Set("User-Agent", "voyagergate-update-check")
		resp, err := client.Do(req)
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
		var rel githubRelease
		if err := json.Unmarshal(body, &rel); err != nil {
			lastErr = err
			continue
		}
		note := strings.TrimSpace(rel.Body)
		if note != "" {
			return note
		}
		// body 为空时退而取 release 页面地址作为说明
		if rel.HTMLURL != "" {
			return rel.HTMLURL
		}
	}
	_ = lastErr
	return ""
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

// normalizeVersion 统一去除版本号前缀 v/V 并 trim
func normalizeVersion(v RemoteVersion) RemoteVersion {
	v.Version = strings.TrimSpace(v.Version)
	v.Version = strings.TrimPrefix(v.Version, "v")
	v.Version = strings.TrimPrefix(v.Version, "V")
	return v
}
