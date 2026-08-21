// Eureka 注册中心适配器。
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"voyagergate/internal/model"
)

const eurekaTimeout = 4 * time.Second

// eurekaClient Eureka 适配器实现
type eurekaClient struct {
	reg *model.Registry
	hc  *http.Client
}

func newEureka(reg *model.Registry) *eurekaClient {
	return &eurekaClient{reg: reg, hc: &http.Client{Timeout: eurekaTimeout}}
}

func (e *eurekaClient) Type() string { return model.TypeEureka }

func (e *eurekaClient) base() string {
	raw := normalizeAddr(strings.TrimSpace(e.reg.Addr), 8761)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	// Eureka clients commonly receive either the server root or its context
	// path. The REST applications endpoint lives below /eureka/apps by default.
	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		path = "/eureka"
	}
	u.Path = path
	return strings.TrimRight(u.String(), "/")
}

// fetchApps 拉取全部应用（instances）
// 兼容两种部署形态：REST 接口在根路径 /apps，或在 /eureka/apps（Spring Cloud 默认 context path）。
func (e *eurekaClient) fetchApps() (*eurekaApplications, error) {
	base := strings.TrimRight(e.base(), "/")
	var lastErr error
	for _, suffix := range []string{"/apps", "/eureka/apps"} {
		raw := base + suffix
		req, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		if e.reg.User != "" {
			req.SetBasicAuth(e.reg.User, e.reg.Pass)
		}
		resp, err := e.hc.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("eureka http %d: %s", resp.StatusCode, truncate(string(body), 120))
			continue
		}
		var apps eurekaApplications
		if err := json.Unmarshal(body, &apps); err != nil {
			lastErr = fmt.Errorf("eureka 响应解析失败: %v", err)
			continue
		}
		return &apps, nil
	}
	return nil, lastErr
}

// eurekaApplications 响应外层结构
type eurekaApplications struct {
	Applications eurekaApps `json:"applications"`
}

type eurekaApps struct {
	Application []eurekaApp `json:"application"`
}

type eurekaApp struct {
	Name     string           `json:"name"`
	Instance []eurekaInstance `json:"instance"`
}

type eurekaInstance struct {
	App    string     `json:"app"`
	IPAddr string     `json:"ipAddr"`
	Port   eurekaPort `json:"port"`
	Status string     `json:"status"`
}

type eurekaPort struct {
	Value   flexibleInt  `json:"$"`
	Enabled flexibleBool `json:"@enabled"`
}

// Eureka servers do not agree on the JSON types for port metadata. Some
// versions return numbers/booleans while others serialize both as strings.
type flexibleInt int

func (v *flexibleInt) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*v = flexibleInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("invalid integer %s", string(data))
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fmt.Errorf("invalid integer %q", s)
	}
	*v = flexibleInt(n)
	return nil
}

type flexibleBool bool

func (v *flexibleBool) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*v = flexibleBool(b)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("invalid boolean %s", string(data))
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes":
		*v = true
	case "false", "0", "no", "":
		*v = false
	default:
		return fmt.Errorf("invalid boolean %q", s)
	}
	return nil
}

// Test 连通性测试：全量拉取即视为连接成功
func (e *eurekaClient) Test() error {
	_, err := e.fetchApps()
	return err
}

// FetchServices 拉取服务列表；端口取第一个启用且 UP 实例的端口
func (e *eurekaClient) FetchServices() ([]*model.Service, error) {
	apps, err := e.fetchApps()
	if err != nil {
		return nil, err
	}
	var out []*model.Service
	for _, app := range apps.Applications.Application {
		name := app.Name
		if name == "" && len(app.Instance) > 0 {
			name = app.Instance[0].App
		}
		if name == "" {
			continue
		}
		svc := &model.Service{Name: name, Src: model.SrcRegistry, On: true}
		for _, inst := range app.Instance {
			if inst.Status == "UP" && bool(inst.Port.Enabled) && int(inst.Port.Value) > 0 {
				svc.Port = int(inst.Port.Value)
				break
			}
		}
		out = append(out, svc)
	}
	return out, nil
}
