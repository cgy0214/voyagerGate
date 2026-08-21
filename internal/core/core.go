package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"

	"voyagergate/internal/config"
	"voyagergate/internal/model"
	"voyagergate/internal/proxy"
	"voyagergate/internal/registry"
	"voyagergate/internal/version"
)

const (
	maxLogs       = 50              // 日志环形队列上限（超过 50 条自动清顶）
	probeInterval = 5 * time.Second // 健康探测周期（自动探测 5s）
)

// Snapshot 前端一次性全量状态
type Snapshot struct {
	Envs     []*model.Environment `json:"envs"`
	Current  string               `json:"current"`
	Theme    string               `json:"theme"`
	Running  bool                 `json:"running"`
	LocalIP  string               `json:"localIp"`
	ReqCount int64                `json:"reqCount"`
	AvgMs    int                  `json:"avgMs"`
	LastSync string               `json:"lastSync"`
	Logs     []*model.LogEntry    `json:"logs"`
}

// Stats 请求统计增量
type Stats struct {
	ReqCount int64 `json:"reqCount"`
	AvgMs    int   `json:"avgMs"`
}

// ConnectResult 连接并拉取结果
type ConnectResult struct {
	Count   int    `json:"count"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// CheckResult 网关连通性检测结果
type CheckResult struct {
	OK      bool   `json:"ok"`
	Ms      int    `json:"ms"`
	Message string `json:"message"`
}

// Core 应用核心服务
type Core struct {
	ctx     context.Context // Wails 上下文（事件推送 / 文件对话框）
	store   *config.Store
	proxy   *proxy.Engine
	proxies map[string]*proxy.Engine

	mu       sync.RWMutex // 保护以下全部业务状态
	envs     []*model.Environment
	current  string
	theme    string
	localIP  string
	reqCount int64
	avgMs    int
	lastSync string
	logs     []*model.LogEntry // 环形队列（新日志在头部）

	timerMu sync.Mutex // 保护定时器生命周期
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewCore 创建核心服务并加载磁盘配置；首次运行生成一个空白环境
func NewCore() *Core {
	return NewCoreWithStore(config.NewStore())
}

// NewCoreWithStore 使用指定配置仓库创建核心服务（便于测试注入临时路径）
func NewCoreWithStore(st *config.Store) *Core {
	f := st.Load()
	c := &Core{
		store:   st,
		proxy:   proxy.New(),
		proxies: make(map[string]*proxy.Engine),
		envs:    f.Envs,
		current: f.Current,
		theme:   f.Theme,
		logs:    []*model.LogEntry{},
	}
	if c.current == "" && len(c.envs) > 0 {
		c.current = c.envs[0].Name
	}
	if len(c.envs) == 0 {
		env := config.DefaultEnv("default")
		c.envs = append(c.envs, env)
		c.current = env.Name
	}
	c.localIP = DetectLocalIP()
	c.proxy.SetLogFn(c.onRequest)
	for _, env := range c.envs {
		for _, s := range env.Services {
			if s.Prefix == "/"+s.Name {
				s.Prefix = ""
			}
		}
		for _, r := range env.Rules {
			r.Dest = normalizeDest(r.Dest)
		}
	}
	for _, env := range c.envs {
		e := proxy.New()
		e.SetLogFn(c.onRequest)
		c.proxies[env.Name] = e
	}
	// 确保配置首次即落盘（启动自动保存）
	c.persist()
	return c
}

func (c *Core) currentProxyLocked() *proxy.Engine {
	if e := c.proxies[c.current]; e != nil {
		return e
	}
	e := proxy.New()
	e.SetLogFn(c.onRequest)
	c.proxies[c.current] = e
	return e
}

// SetCtx 注入 Wails 运行时上下文（startup 时调用）
func (c *Core) SetCtx(ctx context.Context) {
	c.ctx = ctx
}

func (c *Core) Startup(ctx context.Context) {
	c.SetCtx(ctx)
	c.syncProxy()
	c.applyWindowTheme()
	c.AutoBootstrap()
}

func (c *Core) applyWindowTheme() {
	if c.ctx == nil {
		return
	}
	c.mu.RLock()
	theme := c.theme
	c.mu.RUnlock()
	if theme == "light" {
		wailsRuntime.WindowSetLightTheme(c.ctx)
	} else {
		wailsRuntime.WindowSetDarkTheme(c.ctx)
	}
}

func (c *Core) AutoBootstrap() {
	go func() {
		_, _ = c.CheckGateway()
		if err := c.ConnectRegistry(); err == nil {
			_, _ = c.PullServices()
		}
	}()
}

// cur 返回当前环境（假定 c.mu 已持有）
func (c *Core) cur() *model.Environment {
	for _, e := range c.envs {
		if e.Name == c.current {
			return e
		}
	}
	if len(c.envs) > 0 {
		return c.envs[0]
	}
	return nil
}

// buildFile 由内存状态生成磁盘文件结构（假定 c.mu 已持有）
func (c *Core) buildFile() *config.File {
	return &config.File{
		Version: config.Version,
		Theme:   c.theme,
		Current: c.current,
		Envs:    c.envs,
	}
}

// buildState 构建不可变路由快照（深拷贝服务与规则，杜绝并发读写竞争）
func (c *Core) buildState() *proxy.State {
	env := c.cur()
	st := &proxy.State{
		EnvName:       env.Name,
		GW:            env.GW,
		DefaultTarget: env.DefaultTarget,
		DefaultAddr:   env.DefaultAddr,
		Services:      make(map[string]*model.Service),
	}
	for _, r := range env.Rules {
		nr := *r
		st.Rules = append(st.Rules, &nr)
	}
	for _, s := range env.Services {
		ns := *s
		st.Services[s.Name] = &ns
	}
	return st
}

// persist 将内存状态写盘（原子写入，失败仅记录日志不影响运行）
func (c *Core) persist() {
	c.mu.RLock()
	f := c.buildFile()
	c.mu.RUnlock()
	if err := c.store.Save(f); err != nil {
		log.Printf("配置保存失败: %v", err)
	}
}

// syncProxy 将当前环境同步到代理引擎（无停机热更新）
func (c *Core) syncProxy() {
	c.mu.RLock()
	st := c.buildState()
	c.mu.RUnlock()
	c.mu.Lock()
	e := c.currentProxyLocked()
	c.mu.Unlock()
	e.Update(st)
}

// isRunning 代理是否运行
func (c *Core) isRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	env := c.cur()
	return env != nil && env.Running
}

// ---------------------------------------------------------------------------
// 事件推送
// ---------------------------------------------------------------------------

func (c *Core) emitSnapshot() {
	if c.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(c.ctx, "app:snapshot", c.Snapshot())
}

func (c *Core) emitLog(e *model.LogEntry) {
	if c.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(c.ctx, "app:log", e)
}

func (c *Core) emitStats() {
	if c.ctx == nil {
		return
	}
	c.mu.RLock()
	s := &Stats{ReqCount: c.reqCount, AvgMs: c.avgMs}
	c.mu.RUnlock()
	wailsRuntime.EventsEmit(c.ctx, "app:stats", s)
}

// ---------------------------------------------------------------------------
// 日志 & 统计
// ---------------------------------------------------------------------------

// onRequest 代理真实请求回调 → 入队 + 统计 + 推送
func (c *Core) onRequest(e *model.LogEntry) {
	c.pushLog(e)
}

// pushLog 日志入队（环形 50 条）+ 累计统计
func (c *Core) pushLog(e *model.LogEntry) {
	c.mu.Lock()
	c.logs = append([]*model.LogEntry{e}, c.logs...)
	if len(c.logs) > maxLogs {
		c.logs = c.logs[:maxLogs]
	}
	c.reqCount++
	if c.avgMs == 0 {
		c.avgMs = e.MS
	} else {
		c.avgMs = (c.avgMs*19 + e.MS) / 20 // 窗口期 20 条滚动平均
	}
	c.mu.Unlock()
	c.emitLog(e)
	c.emitStats()
}

// GetLogs 返回当前日志队列（新→旧）
func (c *Core) GetLogs() []*model.LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*model.LogEntry, len(c.logs))
	copy(out, c.logs)
	return out
}

// ---------------------------------------------------------------------------
// 健康探测
// ---------------------------------------------------------------------------

// probeNow 对当前环境全部服务执行健康探测并刷新同步时间
func (c *Core) probeNow() {
	c.mu.RLock()
	env := c.cur()
	lastSync := time.Now().Format("15:04:05")
	c.mu.RUnlock()
	if env == nil {
		return
	}
	for _, s := range env.Services {
		addr := s.LocalAddr()
		if addr == "" {
			// 无本地目标 → 无法真实探测，维持小概率随机模拟
			s.Online = proxy.MaybeFlake(s.Online)
			continue
		}
		s.Online = proxy.ProbeTCP(addr)
	}
	c.mu.Lock()
	c.lastSync = lastSync
	c.mu.Unlock()
	c.emitSnapshot()
}

// ---------------------------------------------------------------------------
// 定时任务
// ---------------------------------------------------------------------------

func (c *Core) startTimers() {
	c.timerMu.Lock()
	defer c.timerMu.Unlock()
	if c.stopCh != nil {
		return
	}
	ch := make(chan struct{})
	c.stopCh = ch
	c.wg.Add(1)
	go c.loop(ch)
}

func (c *Core) stopTimers() {
	c.timerMu.Lock()
	ch := c.stopCh
	c.stopCh = nil
	c.timerMu.Unlock()
	if ch != nil {
		close(ch)
	}
	c.wg.Wait()
}

// probeSeconds 当前环境探测间隔（秒），未配置时默认 5s
func (c *Core) probeSeconds() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if env := c.cur(); env != nil && env.Probe > 0 {
		return env.Probe
	}
	return int(probeInterval / time.Second)
}

// SetProbeInterval 设置健康探测间隔（秒，范围 1-3600）
func (c *Core) SetProbeInterval(sec int) error {
	if sec < 1 || sec > 3600 {
		return fmt.Errorf("探测间隔无效: %d 秒（有效范围 1-3600）", sec)
	}
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	env.Probe = sec
	c.mu.Unlock()
	c.persist()
	c.emitSnapshot()
	return nil
}

// loop 周期任务：健康探测（间隔可配置，下一轮生效）
func (c *Core) loop(stop chan struct{}) {
	defer c.wg.Done()
	for {
		probeTick := time.NewTicker(time.Duration(c.probeSeconds()) * time.Second)
		select {
		case <-stop:
			probeTick.Stop()
			return
		case <-probeTick.C:
			if c.isRunning() {
				c.probeNow()
			}
		}
		probeTick.Stop()
	}
}

// ---------------------------------------------------------------------------
// 对外方法（Wails bindings）
// ---------------------------------------------------------------------------

// Snapshot 返回前端全量状态
func (c *Core) Snapshot() *Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	envs := make([]*model.Environment, len(c.envs))
	for i, e := range c.envs {
		envs[i] = deepCopyEnv(e)
	}
	logs := make([]*model.LogEntry, len(c.logs))
	copy(logs, c.logs)
	return &Snapshot{
		Envs:     envs,
		Current:  c.current,
		Theme:    c.theme,
		Running:  func() bool { e := c.cur(); return e != nil && e.Running }(),
		LocalIP:  c.localIP,
		ReqCount: c.reqCount,
		AvgMs:    c.avgMs,
		LastSync: c.lastSync,
		Logs:     logs,
	}
}

// AddEnvironment 新建环境（reg 为前端按类型组装好的注册中心配置）
func (c *Core) AddEnvironment(name string, reg *model.Registry, gw string, port int, copyFrom string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("环境名称必填")
	}
	c.mu.Lock()
	for _, e := range c.envs {
		if e.Name == name {
			c.mu.Unlock()
			return fmt.Errorf("环境已存在: %s", name)
		}
	}
	if len(c.envs) >= config.MaxEnvs {
		c.mu.Unlock()
		return fmt.Errorf("环境数量已达上限 %d", config.MaxEnvs)
	}
	env := config.DefaultEnv(name)
	env.Port = port
	env.GW = gw
	if reg != nil {
		env.Reg = reg
	}
	if copyFrom != "" {
		for _, e := range c.envs {
			if e.Name == copyFrom {
				for _, s := range e.Services {
					env.Services = append(env.Services, deepCopyService(s))
				}
				for _, r := range e.Rules {
					nr := *r
					env.Rules = append(env.Rules, &nr)
				}
				break
			}
		}
	}
	c.envs = append(c.envs, env)
	c.current = name
	newPort := env.Port
	c.mu.Unlock()

	c.syncProxy()
	c.mu.Lock()
	pe := c.currentProxyLocked()
	c.mu.Unlock()
	if pe.Running() {
		if err := pe.Restart(newPort); err != nil {
			c.mu.Lock()
			if env := c.cur(); env != nil {
				env.Running = false
			}
			c.mu.Unlock()
			c.stopTimers()
			return fmt.Errorf("代理重启失败: %v", err)
		}
	}
	c.persist()
	c.emitSnapshot()
	return nil
}

// RemoveEnvironment 删除环境（至少保留一个）
func (c *Core) RemoveEnvironment(name string) error {
	c.mu.Lock()
	if len(c.envs) <= 1 {
		c.mu.Unlock()
		return fmt.Errorf("至少保留一个环境")
	}
	idx := -1
	for i, e := range c.envs {
		if e.Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		c.mu.Unlock()
		return fmt.Errorf("环境不存在: %s", name)
	}
	removedCurrent := c.current == name
	c.envs = append(c.envs[:idx], c.envs[idx+1:]...)
	c.current = c.envs[0].Name
	c.mu.Unlock()

	if removedCurrent {
		c.stopTimers()
		if pe := c.proxies[name]; pe != nil {
			pe.Stop()
			delete(c.proxies, name)
		}
	} else {
		c.syncProxy()
	}
	c.persist()
	c.emitSnapshot()
	return nil
}

// SwitchEnvironment 切换当前环境（整套配置秒切）
func (c *Core) SwitchEnvironment(name string) error {
	c.mu.Lock()
	found := false
	for _, e := range c.envs {
		if e.Name == name {
			found = true
			break
		}
	}
	if !found {
		c.mu.Unlock()
		return fmt.Errorf("环境不存在: %s", name)
	}
	c.current = name
	newPort := c.cur().Port
	c.mu.Unlock()

	c.syncProxy()
	c.mu.Lock()
	pe := c.currentProxyLocked()
	c.mu.Unlock()
	if pe.Running() {
		if err := pe.Restart(newPort); err != nil {
			c.mu.Lock()
			if env := c.cur(); env != nil {
				env.Running = false
			}
			c.mu.Unlock()
			c.stopTimers()
			return fmt.Errorf("切换环境后代理重启失败: %v", err)
		}
	}
	c.persist()
	c.emitSnapshot()
	// 需求：切换空间后连接注册中心并检查网关（与启动时逻辑一致）
	c.AutoBootstrap()
	return nil
}

// SaveRegistry 保存当前环境注册中心配置（字段变更后需重新连接）
func (c *Core) SaveRegistry(reg *model.Registry) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	if reg != nil {
		reg.OK = false
		env.Reg = reg
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

func (c *Core) SetGateway(addr string) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	env.GW = strings.TrimSpace(addr)
	env.GWOk = false
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// SetDefaultTarget 设置未匹配服务时的默认转发目标：
func (c *Core) SetDefaultTarget(target, addr string) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	if target == "local" && addr != "" {
		if err := c.validateForwardAddr(addr, env.Port); err != nil {
			c.mu.Unlock()
			return err
		}
		env.DefaultTarget = "local"
		env.DefaultAddr = addr
	} else {
		env.DefaultTarget = "gateway"
		env.DefaultAddr = ""
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// validateForwardAddr 校验默认转发地址：
//   - 必须是 主机:端口（端口 1-65535）
//   - 端口等于代理监听端口且主机指向本机 → 拒绝（死循环防护）
func (c *Core) validateForwardAddr(addr string, listenPort int) error {
	host, p, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fmt.Errorf("转发地址格式错误，应为 主机:端口，如 127.0.0.1:60018")
	}
	port, e := strconv.Atoi(p)
	if e != nil || port < 1 || port > 65535 {
		return fmt.Errorf("转发地址端口无效: %s", p)
	}
	if port != listenPort {
		return nil
	}
	// 端口与本代理监听一致 → 仅当主机指向本机时才是死循环
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "" || host == "localhost" {
		return fmt.Errorf("转发地址 %s 指向本软件监听端口 %d，会造成回环访问，已拒绝", addr, listenPort)
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return fmt.Errorf("转发地址 %s 指向本软件监听端口 %d，会造成回环访问，已拒绝", addr, listenPort)
		}
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ipn, ok := a.(*net.IPNet); ok && ipn.IP.Equal(ip) {
					return fmt.Errorf("转发地址 %s 指向本软件监听端口 %d，会造成回环访问，已拒绝", addr, listenPort)
				}
			}
		}
	}
	return nil
}

func (c *Core) ResetConfig() error {
	c.mu.Lock()
	if pe := c.currentProxyLocked(); pe.Running() {
		pe.Stop()
	}
	env := config.DefaultEnv("dev-本地")
	c.envs = []*model.Environment{env}
	c.current = env.Name
	c.theme = "dark"
	c.logs = []*model.LogEntry{}
	c.reqCount = 0
	c.avgMs = 0
	c.lastSync = ""
	c.proxies = map[string]*proxy.Engine{}
	c.proxies[env.Name] = proxy.New()
	c.proxies[env.Name].SetLogFn(c.onRequest)
	c.mu.Unlock()
	c.applyWindowTheme()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// SetPort 修改当前环境代理端口（运行中则热重启监听）
func (c *Core) SetPort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("端口无效: %d", port)
	}
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	env.Port = port
	c.mu.Unlock()

	c.mu.Lock()
	pe := c.currentProxyLocked()
	c.mu.Unlock()
	if pe.Running() {
		if err := pe.Restart(port); err != nil {
			c.mu.Lock()
			if env := c.cur(); env != nil {
				env.Running = false
			}
			c.mu.Unlock()
			c.stopTimers()
			return fmt.Errorf("端口变更后代理重启失败: %v", err)
		}
	}
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// ConnectRegistry 连接注册中心（仅测试连通性，成功置 ok）
func (c *Core) ConnectRegistry() error {
	c.mu.RLock()
	env := c.cur()
	if env == nil {
		c.mu.RUnlock()
		return fmt.Errorf("无可用环境")
	}
	regCfg := *env.Reg
	c.mu.RUnlock()

	ad, err := registry.New(&regCfg)
	if err != nil {
		return err
	}
	if err := ad.Test(); err != nil {
		c.mu.Lock()
		env.Reg.OK = false
		c.mu.Unlock()
		c.emitSnapshot()
		return fmt.Errorf("连接失败: %v", err)
	}
	c.mu.Lock()
	env.Reg.OK = true
	c.mu.Unlock()
	c.emitSnapshot()
	return nil
}

// PullServices 从注册中心拉取服务列表并合并（连接成功后调用）
func (c *Core) PullServices() (*ConnectResult, error) {
	c.mu.RLock()
	env := c.cur()
	if env == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("无可用环境")
	}
	regCfg := *env.Reg
	c.mu.RUnlock()

	ad, err := registry.New(&regCfg)
	if err != nil {
		return nil, err
	}
	services, err := ad.FetchServices()
	if err != nil {
		return nil, fmt.Errorf("拉取失败: %v", err)
	}
	c.mu.Lock()
	env.Reg.OK = true
	env.Services = mergeRegistry(env.Services, services)
	for _, s := range env.Services {
		if len(env.RulesOf(s.Name)) == 0 {
			env.Rules = append(env.Rules, &model.Rule{Svc: s.Name, Path: "/**", Dest: model.DestLocal, Prio: 99, On: true})
		}
	}
	c.mu.Unlock()
	c.probeNow()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return &ConnectResult{Count: len(services), OK: true, Message: fmt.Sprintf("连接成功 · 已拉取 %d 个服务", len(services))}, nil
}

// mergeRegistry 合并拉取结果：手动服务始终保留，注册中心服务被拉取覆盖（保留原启用状态）
func mergeRegistry(existing, pulled []*model.Service) []*model.Service {
	var out []*model.Service
	for _, s := range existing {
		if s.Src == model.SrcManual {
			out = append(out, s)
		}
	}
	for _, p := range pulled {
		for _, o := range existing {
			if o.Src == model.SrcRegistry && o.Name == p.Name {
				p.On = o.On
				break
			}
		}
		out = append(out, p)
	}
	return out
}

func (c *Core) CheckGateway() (*CheckResult, error) {
	c.mu.RLock()
	env := c.cur()
	c.mu.RUnlock()
	if env == nil || strings.TrimSpace(env.GW) == "" {
		return &CheckResult{OK: false, Message: "网关地址为空"}, nil
	}
	start := time.Now()
	cli := &http.Client{Timeout: 3 * time.Second}
	resp, err := cli.Get(strings.TrimSpace(env.GW))
	ms := int(time.Since(start).Milliseconds())
	if err != nil {
		c.mu.Lock()
		env.GWOk = false
		c.mu.Unlock()
		c.emitSnapshot()
		return &CheckResult{OK: false, Ms: ms, Message: "✗ 连接失败: " + err.Error()}, nil
	}
	_ = resp.Body.Close()
	c.mu.Lock()
	env.GWOk = true
	c.mu.Unlock()
	c.emitSnapshot()
	return &CheckResult{OK: true, Ms: ms, Message: fmt.Sprintf("✓ 连通 (延迟 %dms)", ms)}, nil
}

// AddService 手动添加服务（服务名必填、不可重复）
func (c *Core) AddService(name, addr, remark string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("服务名必填")
	}
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	if env.FindService(name) != nil {
		c.mu.Unlock()
		return fmt.Errorf("服务已存在: %s", name)
	}
	port := 0
	addr = strings.TrimSpace(addr)
	if addr != "" {
		if _, p, err := net.SplitHostPort(addr); err == nil {
			if n, e := strconv.Atoi(p); e == nil {
				port = n
			}
		}
	}
	s := &model.Service{Name: name, Src: model.SrcManual, On: true, Addr: addr, Port: port, Remark: remark, Prefix: "/" + name}
	if s.LocalAddr() != "" {
		s.Online = proxy.ProbeTCP(s.LocalAddr())
	} else {
		s.Online = false
	}
	env.Services = append(env.Services, s)
	if len(env.RulesOf(name)) == 0 {
		env.Rules = append(env.Rules, &model.Rule{Svc: name, Path: "/**", Dest: model.DestLocal, Prio: 99, On: true})
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// ToggleService 切换服务启用状态（即时生效并写盘）
func (c *Core) ToggleService(name string) {
	c.mu.Lock()
	if env := c.cur(); env != nil {
		if s := env.FindService(name); s != nil {
			s.On = !s.On
		}
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
}

// SetAllServices 批量启用 / 禁用当前环境全部服务
func (c *Core) SetAllServices(on bool) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	for _, s := range env.Services {
		s.On = on
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// DeleteService 删除单个服务
func (c *Core) DeleteService(name string) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	s := env.FindService(name)
	if s == nil {
		c.mu.Unlock()
		return fmt.Errorf("服务不存在: %s", name)
	}
	if s.Src != model.SrcManual {
		c.mu.Unlock()
		return fmt.Errorf("注册中心拉取的服务不可手动删除")
	}
	env.Services = filterServices(env.Services, name)
	env.Rules = filterRules(env.Rules, name)
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// DeleteManualServices 批量删除全部手动添加的服务
func (c *Core) DeleteManualServices() error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	var kept []*model.Service
	for _, s := range env.Services {
		if s.Src != model.SrcManual {
			kept = append(kept, s)
		}
	}
	env.Services = kept
	var keptRules []*model.Rule
	for _, r := range env.Rules {
		if r == nil {
			continue
		}
		if env.FindService(r.Svc) == nil {
			continue
		}
		keptRules = append(keptRules, r)
	}
	env.Rules = keptRules
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// filterServices 移除指定服务
func filterServices(list []*model.Service, name string) []*model.Service {
	out := make([]*model.Service, 0, len(list))
	for _, s := range list {
		if s != nil && s.Name != name {
			out = append(out, s)
		}
	}
	return out
}

// filterRules 移除关联指定服务的全部规则
func filterRules(list []*model.Rule, svc string) []*model.Rule {
	out := make([]*model.Rule, 0, len(list))
	for _, r := range list {
		if r != nil && r.Svc != svc {
			out = append(out, r)
		}
	}
	return out
}

// ruleAt 定位某服务第 idx 条规则（在 c.mu 已持有前提下调用）
func (c *Core) ruleAt(env *model.Environment, svc string, idx int) *model.Rule {
	list := env.RulesOf(svc)
	if idx < 0 || idx >= len(list) {
		return nil
	}
	return list[idx]
}

// AddRule 添加路由规则（保存后即时生效）
// normalizeDest 将目标值归一为统一中文常量（容错英文/大小写变体），未识别时默认本地转发
func normalizeDest(dest string) string {
	d := strings.TrimSpace(dest)
	if d == "" || d == model.DestLocal || strings.EqualFold(d, "local") {
		return model.DestLocal
	}
	if d == model.DestTest || strings.EqualFold(d, "gateway") || strings.EqualFold(d, "gw") || strings.EqualFold(d, "test") {
		return model.DestTest
	}
	return model.DestLocal
}

func (c *Core) AddRule(svc, path, dest string, prio int, remark, stripPrefix string) error {
	if svc == "" {
		return fmt.Errorf("服务不能为空")
	}
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	if env.FindService(svc) == nil {
		c.mu.Unlock()
		return fmt.Errorf("服务不存在: %s", svc)
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/**"
	}
	dest = normalizeDest(dest)
	if prio <= 0 {
		prio = 1
	}
	env.Rules = append(env.Rules, &model.Rule{
		Svc: svc, Path: path, Dest: dest, Prio: prio, Remark: remark, On: true,
		StripPrefix: strings.TrimSpace(stripPrefix),
	})
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// ToggleRule 切换规则启用状态
func (c *Core) ToggleRule(svc string, idx int) {
	c.mu.Lock()
	if env := c.cur(); env != nil {
		if r := c.ruleAt(env, svc, idx); r != nil {
			r.On = !r.On
		}
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
}

// EditRule 编辑规则（path/dest/prio/remark/stripPrefix）
func (c *Core) EditRule(svc string, idx int, path, dest string, prio int, remark, stripPrefix string) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	r := c.ruleAt(env, svc, idx)
	if r == nil {
		c.mu.Unlock()
		return fmt.Errorf("规则不存在")
	}
	path = strings.TrimSpace(path)
	if path != "" {
		r.Path = path
	}
	if dest != "" {
		r.Dest = normalizeDest(dest)
	}
	if prio > 0 {
		r.Prio = prio
	}
	r.Remark = remark
	r.StripPrefix = strings.TrimSpace(stripPrefix)
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// ReorderRules 拖拽重排某服务规则顺序，并按新顺序重设优先级（1..N）。
// order 为新顺序对应的原规则索引序列（前端按展示列表顺序给出）。
func (c *Core) ReorderRules(svc string, order []int) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	sub := env.RulesOf(svc)
	if len(order) != len(sub) {
		c.mu.Unlock()
		return fmt.Errorf("规则数量不一致")
	}
	reordered := make([]*model.Rule, 0, len(sub))
	for _, idx := range order {
		if idx < 0 || idx >= len(sub) {
			c.mu.Unlock()
			return fmt.Errorf("非法规则索引: %d", idx)
		}
		reordered = append(reordered, sub[idx])
	}
	// 按新顺序重建 env.Rules（其他服务的规则保持原位）
	pos := 0
	for i, r := range env.Rules {
		if r.Svc == svc {
			env.Rules[i] = reordered[pos]
			pos++
		}
	}
	// 重设优先级：数字越小越优先
	for i, r := range reordered {
		r.Prio = i + 1
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// SetServicePrefix 批量设置服务前缀：写入 Service.Prefix 并应用到该服务全部规则（空 = 清除去前缀）
func (c *Core) SetServicePrefix(svc, prefix string) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	s := env.FindService(svc)
	if s == nil {
		c.mu.Unlock()
		return fmt.Errorf("服务不存在: %s", svc)
	}
	prefix = strings.TrimSpace(strings.Trim(prefix, "/"))
	if prefix != "" {
		prefix = "/" + prefix
	}
	s.Prefix = prefix
	for _, r := range env.RulesOf(svc) {
		r.StripPrefix = prefix
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// ToggleRuleStrip 切换单条规则是否本地转发时去除前缀（按服务前缀值）
func (c *Core) ToggleRuleStrip(svc string, idx int) {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return
	}
	r := c.ruleAt(env, svc, idx)
	s := env.FindService(svc)
	if r == nil || s == nil {
		c.mu.Unlock()
		return
	}
	if r.StripPrefix == "" {
		prefix := s.Prefix
		if prefix == "" {
			prefix = "/" + strings.ToLower(svc)
		}
		r.StripPrefix = prefix
	} else {
		r.StripPrefix = ""
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
}

// SetAllRules 批量启用 / 禁用某服务的全部规则
func (c *Core) SetAllRules(svc string, on bool) error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	if env.FindService(svc) == nil {
		c.mu.Unlock()
		return fmt.Errorf("服务不存在: %s", svc)
	}
	for _, r := range env.RulesOf(svc) {
		r.On = on
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// DeleteRule 删除规则
func (c *Core) DeleteRule(svc string, idx int) {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return
	}
	r := c.ruleAt(env, svc, idx)
	if r == nil {
		c.mu.Unlock()
		return
	}
	for i, x := range env.Rules {
		if x == r {
			env.Rules = append(env.Rules[:i], env.Rules[i+1:]...)
			break
		}
	}
	c.mu.Unlock()
	c.syncProxy()
	c.persist()
	c.emitSnapshot()
}

// StartProxy 启动代理（当前环境端口）
func (c *Core) StartProxy() error {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return fmt.Errorf("无可用环境")
	}
	if env.Running {
		c.mu.Unlock()
		return nil
	}
	env.Running = true
	port := env.Port
	c.mu.Unlock()

	c.syncProxy()
	c.mu.Lock()
	pe := c.currentProxyLocked()
	c.mu.Unlock()
	if err := pe.Start(port); err != nil {
		c.mu.Lock()
		env.Running = false
		c.mu.Unlock()
		return fmt.Errorf("代理启动失败: %v", err)
	}
	c.startTimers()
	c.emitSnapshot()
	return nil
}

// StopProxy 停止代理
func (c *Core) StopProxy() {
	c.mu.Lock()
	env := c.cur()
	if env == nil {
		c.mu.Unlock()
		return
	}
	env.Running = false
	c.mu.Unlock()
	c.stopTimers()
	if pe := c.proxies[env.Name]; pe != nil {
		pe.Stop()
	}
	c.emitSnapshot()
}

// SetTheme 切换界面主题并记忆（同时切换 Windows 原生标题栏主题）
func (c *Core) SetTheme(theme string) {
	if theme != "dark" && theme != "light" {
		theme = "dark"
	}
	c.mu.Lock()
	c.theme = theme
	c.mu.Unlock()
	c.persist()
	c.emitSnapshot()
	c.applyWindowTheme()
}

// PreviewYAML 生成当前环境 YAML 预览（密码/Token 脱敏）
func (c *Core) PreviewYAML() string {
	c.mu.RLock()
	env := c.cur()
	ip := c.localIP
	c.mu.RUnlock()
	if env == nil {
		return ""
	}
	return config.PreviewYAML(env, ip)
}

// SaveConfig 手动保存配置到磁盘
func (c *Core) SaveConfig() error {
	c.persist()
	return nil
}

// ConfigPath 返回配置文件路径
func (c *Core) ConfigPath() string {
	return config.ConfigPath()
}

// OpenConfigDir 打开配置文件所在目录
func (c *Core) OpenConfigDir() error {
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer", "/select,", filepath.Join(dir, "config.yaml")).Start()
	}
	return nil
}

// ExportEnvironment 导出环境 JSON（团队共享，密码/Token 脱敏）
func (c *Core) ExportEnvironment(name string) (string, error) {
	c.mu.RLock()
	var env *model.Environment
	for _, e := range c.envs {
		if e.Name == name {
			env = e
			break
		}
	}
	c.mu.RUnlock()
	if env == nil {
		return "", fmt.Errorf("环境不存在: %s", name)
	}
	cp := deepCopyEnv(env)
	if cp.Reg.Pass != "" {
		cp.Reg.Pass = model.Mask(cp.Reg.Pass)
	}
	if cp.Reg.Token != "" {
		cp.Reg.Token = model.Mask(cp.Reg.Token)
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(config.ConfigDir(), "exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, safeFileName(name)+".env.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// ExportConfig 导出完整配置快照
func (c *Core) ExportConfig() (string, error) {
	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		return "", err
	}
	dir := filepath.Join(config.ConfigDir(), "exports")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, "config-"+time.Now().Format("20060102-150405")+".yaml")
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

// ImportConfig 通过文件对话框导入配置（替换全部环境）
func (c *Core) ImportConfig() error {
	if c.ctx == nil {
		return fmt.Errorf("运行时上下文未就绪")
	}
	f, err := wailsRuntime.OpenFileDialog(c.ctx, wailsRuntime.OpenDialogOptions{
		Title: "导入 VoyagerGate 配置",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "YAML 配置文件", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return err
	}
	if f == "" {
		return nil // 用户取消
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	var disk config.File
	if err := yaml.Unmarshal(data, &disk); err != nil {
		return fmt.Errorf("配置文件解析失败: %v", err)
	}
	if len(disk.Envs) == 0 {
		return fmt.Errorf("配置文件中无环境")
	}
	c.mu.Lock()
	c.envs = disk.Envs
	for _, env := range c.envs {
		for _, r := range env.Rules {
			r.Dest = normalizeDest(r.Dest)
		}
	}
	if !envExists(c.envs, disk.Current) {
		c.current = c.envs[0].Name
	} else {
		c.current = disk.Current
	}
	if disk.Theme != "" {
		c.theme = disk.Theme
	}
	c.mu.Unlock()

	c.syncProxy()
	c.persist()
	c.emitSnapshot()
	return nil
}

// CheckUpdate 检查更新（逻辑在 internal/version）
func (c *Core) CheckUpdate() string {
	return version.CheckUpdate()
}

// GetVersion 返回当前产品版本号（形如 v1.0.0，供前端展示时自行去 v）
func (c *Core) GetVersion() string {
	return version.Version
}

// CheckUpdateDetail 返回详细更新信息（含版本号/说明/下载地址）
func (c *Core) CheckUpdateDetail() *version.CheckUpdateResult {
	return version.CheckUpdateDetail()
}

// DownloadAndUpdate 下载新版本并替换当前程序
func (c *Core) DownloadAndUpdate(url string) error {
	return version.DownloadAndUpdate(url)
}

// OpenURL 用系统默认浏览器打开指定 URL
func (c *Core) OpenURL(url string) {
	if c.ctx == nil {
		return
	}
	wailsRuntime.BrowserOpenURL(c.ctx, url)
}

// GetLocalIP 返回本机局域网 IPv4
func (c *Core) GetLocalIP() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.localIP
}

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

// DetectLocalIP 探测本机局域网 IPv4（优先私网地址，用于 IP:端口 复制）
func DetectLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	var fallback string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			s := ip4.String()
			if isPrivate(ip4) {
				return s
			}
			if fallback == "" {
				fallback = s
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	return "127.0.0.1"
}

func isPrivate(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip[0] == 10 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}

// envExists 环境是否存在
func envExists(envs []*model.Environment, name string) bool {
	for _, e := range envs {
		if e.Name == name {
			return true
		}
	}
	return false
}

// deepCopyEnv 深拷贝环境（避免快照与运行态共享引用）
func deepCopyEnv(e *model.Environment) *model.Environment {
	if e == nil {
		return nil
	}
	cp := *e
	if e.Reg != nil {
		r := *e.Reg
		cp.Reg = &r
	}
	cp.Services = make([]*model.Service, len(e.Services))
	for i, s := range e.Services {
		cp.Services[i] = deepCopyService(s)
	}
	cp.Rules = make([]*model.Rule, len(e.Rules))
	for i, r := range e.Rules {
		nr := *r
		cp.Rules[i] = &nr
	}
	return &cp
}

func deepCopyService(s *model.Service) *model.Service {
	if s == nil {
		return nil
	}
	cp := *s
	return &cp
}

// safeFileName 过滤文件名非法字符
func safeFileName(name string) string {
	return strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(name)
}
