// Package proxy 实现本地代理核心：原生 net/http 反向代理 + 读写锁热更新。
package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"voyagergate/internal/model"
	"voyagergate/internal/rule"
)

// State 一次性不可变路由快照（每次配置变更由 core 重新构建后整体替换）
type State struct {
	EnvName       string
	GW            string
	DefaultTarget string // 未匹配服务的默认目标：gateway | local（nginx 式兜底）
	DefaultAddr   string // DefaultTarget=local 时的转发地址
	Services      map[string]*model.Service
	Rules         []*model.Rule
	cacheMu sync.Mutex                     // 保护 cache（并发请求下首个访问时创建代理）
	cache   map[string]*httputil.ReverseProxy // 按目标地址缓存代理
}

// Decide 依据命中规则与服务状态输出决策：返回 (决策文案, 目标地址)
// 决策优先级（与需求文档 3.6 语义一致）：
//  1. 服务不存在或服务禁用 → 网关（不判断本地）
//  2. 规则显式指向测试环境 → 网关
//  3. 无本地目标地址 → 网关
//  4. 本地在线 → 本地转发
//  5. 本地离线 → 自动降级
func (st *State) Decide(rt *model.Rule, svc *model.Service) (string, string) {
	if st == nil || st.GW == "" {
		return model.DestTest, ""
	}
	if svc == nil || !svc.On {
		return model.DestTest, st.GW
	}
	if rt != nil && strings.EqualFold(rt.Dest, model.DestTest) {
		return model.DestTest, st.GW
	}
	addr := svc.LocalAddr()
	if addr == "" {
		return model.DestTest, st.GW
	}
	if svc.Online {
		return model.DestLocal, "http://" + addr
	}
	return model.DestTest, st.GW
}

// firstSegment 提取请求路径首段（忽略 query），如 /inventory-server/x → inventory-server
func firstSegment(path string) string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	path = strings.Trim(path, "/")
	if i := strings.IndexByte(path, '/'); i >= 0 {
		path = path[:i]
	}
	return path
}

// rulesOfService 返回指定服务的全部规则
func (st *State) rulesOfService(svc string) []*model.Rule {
	var out []*model.Rule
	for _, r := range st.Rules {
		if r != nil && r.Svc == svc {
			out = append(out, r)
		}
	}
	return out
}

// resolveService 由请求路径推断所属服务（首段 → 服务名 / 前缀 / 规则首段，均忽略大小写）
func (st *State) resolveService(path string) *model.Service {
	seg := firstSegment(path)
	if seg == "" {
		return nil
	}
	// 1. 服务名精确匹配（保持原优先级）
	if s := st.Services[seg]; s != nil {
		return s
	}
	// 2. 服务名忽略大小写匹配（Eureka 注册名常为大写，URL 首段小写）
	for n, s := range st.Services {
		if strings.EqualFold(n, seg) {
			return s
		}
	}
	// 3. 前缀匹配（忽略大小写）
	for _, s := range st.Services {
		if p := strings.Trim(s.Prefix, "/"); p != "" && strings.EqualFold(p, seg) {
			return s
		}
	}
	// 4. 规则首段匹配（忽略大小写）
	for _, r := range st.Rules {
		if r == nil || r.Svc == "" {
			continue
		}
		if p := strings.Trim(r.Path, "/"); p != "" && p != "**" && strings.EqualFold(firstSegment(p), seg) {
			if s := st.Services[r.Svc]; s != nil {
				return s
			}
		}
	}
	return nil
}

// Route 核心决策：由路径推断服务 → 仅应用该服务的规则，输出 (命中规则, 服务, 决策, 目标地址)
func (st *State) Route(path string) (*model.Rule, *model.Service, string, string) {
	if st == nil {
		return nil, nil, model.DestTest, ""
	}
	svc := st.resolveService(path)
	if svc == nil {
		// 未匹配到任何服务：nginx 式默认转发（指定地址直接转发，路径原样保留）
		if st.DefaultTarget == "local" && st.DefaultAddr != "" {
			return nil, nil, model.DestLocal, "http://" + st.DefaultAddr
		}
		if st.GW == "" {
			return nil, nil, model.DestTest, ""
		}
		return nil, nil, model.DestTest, st.GW
	}
	if !svc.On {
		return nil, nil, model.DestTest, st.GW
	}
	rules := st.rulesOfService(svc.Name)
	if len(rules) == 0 {
		if addr := svc.LocalAddr(); addr != "" && svc.Online {
			return nil, svc, model.DestLocal, "http://" + addr
		}
		return nil, svc, model.DestTest, st.GW
	}
	rt := rule.BestRule(rules, path)
	// 规则路径按“服务前缀相对路径”书写（如 /scheduler/x，服务前缀 /inventory-server）：
	// 同时用完整路径与剥前缀后的路径匹配，取优先级更高者，避免相对路径规则落空走网关。
	if svc.Prefix != "" {
		if stripped := rule.StripPrefixPath(path, svc.Prefix); stripped != path {
			if r := rule.BestRule(rules, stripped); r != nil && (rt == nil || r.Prio < rt.Prio) {
				rt = r
			}
		}
	}
	if rt == nil {
		return nil, svc, model.DestTest, st.GW
	}
	dec, target := st.Decide(rt, svc)
	return rt, svc, dec, target
}

// proxyFor 按目标地址获取（并缓存）反向代理实例
func (st *State) proxyFor(target string) *httputil.ReverseProxy {
	if target == "" {
		return nil
	}
	st.cacheMu.Lock()
	defer st.cacheMu.Unlock()
	if st.cache == nil {
		st.cache = make(map[string]*httputil.ReverseProxy)
	}
	if p, ok := st.cache[target]; ok {
		return p
	}
	u, err := url.Parse(target)
	if err != nil {
		log.Printf("目标地址解析失败 %s: %v", target, err)
		return nil
	}
	p := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			// 关键：以目标主机名转发，避免网关按 Host 做虚拟主机路由而返回 404
			r.Out.Host = r.Out.URL.Host
		},
		ErrorLog: nil,
	}
	st.cache[target] = p
	return p
}

// Engine 代理引擎：持有当前状态 + HTTP 服务，支持无停机热更新
type Engine struct {
	mu    sync.RWMutex
	state *State
	srv   *http.Server
	ln    net.Listener
	port  int
	lan   bool // 是否绑定 0.0.0.0（允许局域网访问）；false 仅绑 127.0.0.1
	logFn func(entry *model.LogEntry) // 每次请求完成的日志回调
}

// New 创建代理引擎
func New() *Engine {
	e := &Engine{}
	e.srv = &http.Server{Handler: e}
	return e
}

// SetLogFn 注入日志回调（由 core 提供，写入环形队列并推送前端）
func (e *Engine) SetLogFn(fn func(entry *model.LogEntry)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logFn = fn
}

// Update 无停机替换路由状态（写锁，切换瞬间生效）
func (e *Engine) Update(st *State) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = st
}

// Port 返回当前监听端口
func (e *Engine) Port() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.port
}

// SetLAN 设置是否允许局域网访问（下次 Start/Restart 生效）
func (e *Engine) SetLAN(on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lan = on
}

// bindHost 依据 LAN 开关返回监听地址（默认仅回环，安全优先）
func (e *Engine) bindHost() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.lan {
		return "0.0.0.0:"
	}
	return "127.0.0.1:"
}

// Running 是否正在监听
func (e *Engine) Running() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ln != nil
}

// Start 在指定端口启动监听；端口被占用返回错误
func (e *Engine) Start(port int) error {
	e.mu.Lock()
	if e.ln != nil {
		e.mu.Unlock()
		return nil // 已在运行，忽略重复启动
	}
	e.mu.Unlock()

	ln, err := net.Listen("tcp", e.bindHost()+strconv.Itoa(port))
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.ln = ln
	e.port = port
	e.mu.Unlock()

	go func() {
		if err := e.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("代理服务异常: %v", err)
		}
	}()
	log.Printf("代理已启动，监听端口 %d", port)
	return nil
}

// Stop 停止监听（无停机热更新场景下，停止后新配置可通过 Update + Start 恢复）
func (e *Engine) Stop() {
	e.mu.Lock()
	ln := e.ln
	e.ln = nil
	e.port = 0
	e.mu.Unlock()
	if ln != nil {
		_ = ln.Close()
	}
	log.Printf("代理已停止")
}

// Restart 端口变更时热重启监听（服务端对象复用，零停机）
func (e *Engine) Restart(port int) error {
	e.Stop()
	return e.Start(port)
}

// statusRecorder 捕获响应状态码供日志统计
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// ServeHTTP 核心路由逻辑
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 并发安全读取：状态快照 + 日志回调
	e.mu.RLock()
	st := e.state
	logFn := e.logFn
	e.mu.RUnlock()

	start := time.Now()
	dec := ""
	target := ""
	origPath := r.URL.Path
	var rt *model.Rule
	var svc *model.Service

	if st != nil {
		rt, svc, dec, target = st.Route(r.URL.Path)

		if dec == model.DestLocal && rt != nil && rt.StripPrefix != "" {
			r.URL.Path = rule.StripPrefixPath(r.URL.Path, rt.StripPrefix)
		}
		finalPath := r.URL.Path

		rec := &statusRecorder{ResponseWriter: w, status: 502}
		if target == "" {
			http.Error(rec, "VoyagerGate: 无可用目标（代理未配置网关）", http.StatusBadGateway)
		} else if p := st.proxyFor(target); p != nil {
			p.ServeHTTP(rec, r)
		} else {
			http.Error(rec, "VoyagerGate: 目标地址非法", http.StatusBadGateway)
		}

		if logFn != nil {
			svcName := ""
			if rt != nil {
				svcName = rt.Svc
			} else if svc != nil {
				svcName = svc.Name
			}
			logFn(&model.LogEntry{
				T:      time.Now().Format("15:04:05"),
				M:      r.Method,
				P:      origPath,
				Raw:    origPath,
				Final:  finalPath,
				Target: target,
				S:      svcName,
				Dec:    dec,
				C:      rec.status,
				MS:     int(time.Since(start).Milliseconds()),
				SName:  svcName,
			})
		}
		return
	}

	// 代理未配置状态：直接 502
	http.Error(w, "VoyagerGate: 代理状态未就绪", http.StatusBadGateway)
}
