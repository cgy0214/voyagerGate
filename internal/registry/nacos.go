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

type nacosClient struct {
	reg        *model.Registry
	hc         *http.Client
	accessToken string
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

func (n *nacosClient) isV3() bool {
	return n.reg.NacosVer == "3"
}

func (n *nacosClient) get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	if n.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.accessToken)
	}
	resp, err := n.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("nacos http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func (n *nacosClient) post(rawURL string, data url.Values) ([]byte, error) {
	resp, err := n.hc.PostForm(rawURL, data)
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

func (n *nacosClient) login() error {
	if n.reg.NacosUser == "" {
		return nil
	}
	loginURL := n.base() + "/nacos/v1/auth/login"
	if n.isV3() {
		loginURL = n.base() + "/nacos/v3/auth/user/login"
	}
	body, err := n.post(loginURL, url.Values{
		"username": {n.reg.NacosUser},
		"password": {n.reg.NacosPass},
	})
	if err != nil {
		return fmt.Errorf("nacos 登录失败: %v", err)
	}
	var result struct {
		AccessToken string `json:"accessToken"`
		GlobalAdmin bool   `json:"globalAdmin"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("nacos 登录响应解析失败: %v", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("nacos 登录失败: 未获取到 accessToken")
	}
	n.accessToken = result.AccessToken
	if n.isV3() && !result.GlobalAdmin {
		return fmt.Errorf("nacos v3 拉取服务列表需要管理员权限，当前账号非管理员")
	}
	return nil
}

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

type nacosServiceList struct {
	Doms  []string `json:"doms"`
	Count int      `json:"count"`
}

type nacosV3ServiceListResp struct {
	Code int              `json:"code"`
	Data nacosV3ServiceData `json:"data"`
}

type nacosV3ServiceData struct {
	TotalCount int            `json:"totalCount"`
	PageItems  []nacosV3SvcItem `json:"pageItems"`
}

type nacosV3SvcItem struct {
	Name string `json:"name"`
}

type nacosInstanceList struct {
	Hosts []nacosHost `json:"hosts"`
}

type nacosV3InstanceListResp struct {
	Code int       `json:"code"`
	Data []nacosHost `json:"data"`
}

type nacosHost struct {
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Healthy bool   `json:"healthy"`
}

func (n *nacosClient) Test() error {
	if err := n.login(); err != nil {
		return err
	}
	_, err := n.fetchNames()
	return err
}

func (n *nacosClient) fetchNames() ([]string, error) {
	if n.isV3() {
		return n.fetchNamesV3()
	}
	return n.fetchNamesV1()
}

func (n *nacosClient) fetchNamesV1() ([]string, error) {
	q := n.query(url.Values{
		"pageNo":     {"1"},
		"pageSize":   {"500"},
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

func (n *nacosClient) fetchNamesV3() ([]string, error) {
	q := n.query(url.Values{
		"pageNo":   {"1"},
		"pageSize": {"500"},
	})
	raw := n.base() + "/nacos/v3/admin/ns/service/list?" + q.Encode()
	body, err := n.get(raw)
	if err != nil {
		return nil, err
	}
	var resp nacosV3ServiceListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("nacos 服务列表解析失败: %v", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("nacos 返回错误码: %d", resp.Code)
	}
	var names []string
	for _, svc := range resp.Data.PageItems {
		names = append(names, svc.Name)
	}
	return names, nil
}

func (n *nacosClient) FetchServices() ([]*model.Service, error) {
	if err := n.login(); err != nil {
		return nil, err
	}
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

func (n *nacosClient) firstPort(name string) (int, bool) {
	if n.isV3() {
		return n.firstPortV3(name)
	}
	return n.firstPortV1(name)
}

func (n *nacosClient) firstPortV1(name string) (int, bool) {
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

func (n *nacosClient) firstPortV3(name string) (int, bool) {
	q := n.query(url.Values{"serviceName": {name}})
	raw := n.base() + "/nacos/v3/client/ns/instance/list?" + q.Encode()
	body, err := n.get(raw)
	if err != nil {
		return 0, false
	}
	var resp nacosV3InstanceListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false
	}
	for _, h := range resp.Data {
		if h.Healthy && h.Port > 0 {
			return h.Port, true
		}
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
