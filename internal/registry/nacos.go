// Nacos 注册中心适配器。
package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"voyagergate/internal/model"
)

const (
	nacosDefaultPort = 8848
	nacosTimeout     = 4 * time.Second
)

// nacosClient Nacos 适配器实现
type nacosClient struct {
	reg *model.Registry
	hc  *http.Client
}

func newNacos(reg *model.Registry) *nacosClient {
	return &nacosClient{
		reg: reg,
		hc:  &http.Client{Timeout: nacosTimeout},
	}
}

func (n *nacosClient) Type() string { return model.TypeNacos }

func (n *nacosClient) base() string {
	return normalizeAddr(n.reg.Addr, nacosDefaultPort)
}

// get 发起 GET 请求并返回原始字节（非 2xx 返回错误）
func (n *nacosClient) get(rawURL string) ([]byte, error) {
	resp, err := n.hc.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nacos http %d: %s", resp.StatusCode, truncate(string(body), 120))
	}
	return body, nil
}

// query 拼接带 namespaceId / groupName 的公共参数
func (n *nacosClient) query(extra url.Values) url.Values {
	q := url.Values{}
	if n.reg.NS != "" {
		q.Set("namespaceId", n.reg.NS)
	}
	if n.reg.Group != "" {
		q.Set("groupName", n.reg.Group)
	}
	for k, v := range extra {
		q[k] = v
	}
	return q
}

// nacosServiceList 服务列表响应
type nacosServiceList struct {
	Doms  []string `json:"doms"`
	Count int      `json:"count"`
}

// nacosInstanceList 实例列表响应
type nacosInstanceList struct {
	Hosts []nacosHost `json:"hosts"`
}

type nacosHost struct {
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Healthy bool   `json:"healthy"`
}

// Test 连通性测试：拉取服务列表即视为连接成功
func (n *nacosClient) Test() error {
	_, err := n.fetchNames()
	return err
}

// fetchNames 拉取全部服务名
func (n *nacosClient) fetchNames() ([]string, error) {
	q := n.query(url.Values{
		"pageNo":   {"1"},
		"pageSize": {"500"},
		"hasIpCount": {"true"},
	})
	raw := n.base() + "/nacos/v1/ns/service/list?" + q.Encode()
	body, err := n.get(raw)
	if err != nil {
		return nil, err
	}
	var list nacosServiceList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("nacos 服务列表解析失败: %v", err)
	}
	return list.Doms, nil
}

// FetchServices 拉取服务列表；对每个服务取首个健康实例的端口作为本地开发端口
func (n *nacosClient) FetchServices() ([]*model.Service, error) {
	names, err := n.fetchNames()
	if err != nil {
		return nil, err
	}
	var out []*model.Service
	for _, name := range names {
		svc := &model.Service{Name: name, Src: model.SrcRegistry, On: true}
		if port, ok := n.firstPort(name); ok {
			svc.Port = port
		}
		out = append(out, svc)
	}
	return out, nil
}

// firstPort 返回服务第一个健康实例端口
func (n *nacosClient) firstPort(name string) (int, bool) {
	q := n.query(url.Values{"serviceName": {name}})
	raw := n.base() + "/nacos/v1/ns/instance/list?" + q.Encode()
	body, err := n.get(raw)
	if err != nil {
		return 0, false
	}
	var list nacosInstanceList
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, false
	}
	for _, h := range list.Hosts {
		if h.Healthy && h.Port > 0 {
			return h.Port, true
		}
	}
	return 0, false
}

// truncate 截断长文本便于错误信息展示
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
