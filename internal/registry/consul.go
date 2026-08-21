// Consul 注册中心适配器。
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"voyagergate/internal/model"
)

const (
	consulDefaultPort = 8500
	consulTimeout     = 4 * time.Second
)

// consulClient Consul 适配器实现
type consulClient struct {
	reg *model.Registry
	hc  *http.Client
}

func newConsul(reg *model.Registry) *consulClient {
	return &consulClient{reg: reg, hc: &http.Client{Timeout: consulTimeout}}
}

func (c *consulClient) Type() string { return model.TypeConsul }

func (c *consulClient) base() string { return normalizeAddr(c.reg.Addr, consulDefaultPort) }

// get 带 Token 头的 GET 请求
func (c *consulClient) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if c.reg.Token != "" {
		req.Header.Set("X-Consul-Token", c.reg.Token)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("consul http %d: %s", resp.StatusCode, truncate(string(body), 120))
	}
	return body, nil
}

// dcQuery 数据中心参数（可选）
func (c *consulClient) dcQuery() string {
	if c.reg.DC != "" {
		return "?dc=" + urlQueryEscape(c.reg.DC)
	}
	return ""
}

// Test 连通性测试：查询集群 leader 即视为连接成功
func (c *consulClient) Test() error {
	raw := c.base() + "/v1/status/leader" + c.dcQuery()
	_, err := c.get(raw)
	return err
}

// catalogServices 服务目录响应：serviceName -> tags
type consulCatalogServices map[string][]string

// consulServiceEntry 健康服务响应条目
type consulServiceEntry struct {
	Service struct {
		Service string `json:"Service"`
		Port    int    `json:"Port"`
	} `json:"Service"`
	Checks []struct {
		Status string `json:"Status"`
	} `json:"Checks"`
}

// FetchServices 拉取服务列表；端口取第一个 passing 检查实例的端口
func (c *consulClient) FetchServices() ([]*model.Service, error) {
	raw := c.base() + "/v1/catalog/services" + c.dcQuery()
	body, err := c.get(raw)
	if err != nil {
		return nil, err
	}
	var catalog consulCatalogServices
	if err := json.Unmarshal(body, &catalog); err != nil {
		return nil, fmt.Errorf("consul 服务目录解析失败: %v", err)
	}
	var out []*model.Service
	for name := range catalog {
		svc := &model.Service{Name: name, Src: model.SrcRegistry, On: true}
		if port, ok := c.firstPort(name); ok {
			svc.Port = port
		}
		out = append(out, svc)
	}
	return out, nil
}

// firstPort 返回服务第一个 passing 健康实例端口
func (c *consulClient) firstPort(name string) (int, bool) {
	raw := c.base() + "/v1/health/service/" + urlPathEscape(name) + c.dcQuery()
	body, err := c.get(raw)
	if err != nil {
		return 0, false
	}
	var entries []consulServiceEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return 0, false
	}
	for _, e := range entries {
		passing := true
		for _, chk := range e.Checks {
			if chk.Status != "passing" {
				passing = false
				break
			}
		}
		if passing && e.Service.Port > 0 {
			return e.Service.Port, true
		}
	}
	return 0, false
}

func urlQueryEscape(s string) string { return strings.ReplaceAll(s, " ", "%20") }
func urlPathEscape(s string) string {
	replacer := strings.NewReplacer("/", "%2F", " ", "%20")
	return replacer.Replace(s)
}