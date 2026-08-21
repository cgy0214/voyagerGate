// Package model 定义 VoyagerGate 核心数据模型。
package model

import (
	"strconv"
	"strings"
)

// 注册中心类型常量（本期仅支持 Nacos / Eureka / Consul，K8S 下版本实现）
const (
	TypeNacos  = "Nacos"
	TypeEureka = "Eureka"
	TypeConsul = "Consul"
)

// 服务来源标识
const (
	SrcRegistry = "reg" // 🛰 注册中心拉取
	SrcManual   = "man" // ✍ 手动添加
)

// 规则目标
const (
	DestLocal = "本地"
	DestTest  = "网关"
)

// Registry 注册中心连接配置（类型联动字段，仅填充当前类型相关字段）
type Registry struct {
	Type  string `json:"type" yaml:"type"`                         // Nacos | Eureka | Consul
	Addr  string `json:"addr" yaml:"addr"`                         // 服务地址
	NS    string `json:"ns,omitempty" yaml:"namespace,omitempty"`  // Nacos 命名空间
	Group string `json:"group,omitempty" yaml:"group,omitempty"`   // Nacos 分组
	User  string `json:"user,omitempty" yaml:"username,omitempty"` // Eureka 用户名
	Pass  string `json:"pass,omitempty" yaml:"password,omitempty"` // Eureka 密码
	DC    string `json:"dc,omitempty" yaml:"datacenter,omitempty"` // Consul 数据中心
	Token string `json:"token,omitempty" yaml:"token,omitempty"`   // Consul ACL Token
	OK    bool   `json:"ok" yaml:"-"`                              // 连接成功标志（运行态，不落盘）
}

// Service 服务（本地开发服务与注册中心服务统一建模）
type Service struct {
	Name   string `json:"name" yaml:"name"`                         // 服务名
	Src    string `json:"src" yaml:"source"`                        // 来源：reg | man
	On     bool   `json:"on" yaml:"enabled"`                        // 是否启用（参与转发）
	Online bool   `json:"online" yaml:"-"`                          // 在线状态（运行态，由健康探测决定，不落盘）
	Port   int    `json:"port" yaml:"port"`                         // 本地端口（注册中心拉取 = 实例端口，手动 = 从地址解析）
	Addr   string `json:"addr,omitempty" yaml:"-"`                  // 手动添加的本地地址 127.0.0.1:9106（不落盘，Port 已承载）
	Remark string `json:"remark,omitempty" yaml:"remark,omitempty"` // 备注
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"` // 服务前缀（如 /inventory-server，批量设置到规则后本地转发时去除）
}

// LocalAddr 返回本服务可转发的本地目标地址：
//   - 手动添加：以本地地址为准（空 = 只登记不转发）
//   - 注册中心：本地假定开发端口与注册端口一致 → 127.0.0.1:Port
//     返回空串表示无本地目标（只能降级到网关）。
func (s *Service) LocalAddr() string {
	if s == nil {
		return ""
	}
	if s.Src == SrcManual {
		return s.Addr
	}
	if s.Port > 0 {
		return "127.0.0.1:" + strconv.Itoa(s.Port)
	}
	return ""
}

// Rule 路由规则：按优先级匹配，决定请求去向（本地 / 网关）
type Rule struct {
	Svc         string `json:"svc" yaml:"service"`                                 // 所属服务
	Path        string `json:"path" yaml:"path"`                                   // 匹配路径（支持 /**、*）
	Dest        string `json:"dest" yaml:"dest"`                                   // 目标：本地 | 测试环境
	Prio        int    `json:"prio" yaml:"priority"`                               // 优先级（数字越小越优先）
	Remark      string `json:"remark,omitempty" yaml:"remark,omitempty"`           // 备注
	On          bool   `json:"on" yaml:"enabled"`                                  // 是否启用
	StripPrefix string `json:"stripPrefix,omitempty" yaml:"stripPrefix,omitempty"` // 本地转发时去除的前缀（如 /inventory-server，本地无网关需去除）
}

type Environment struct {
	Name          string     `json:"name" yaml:"name"`                             // 环境名（唯一）
	Port          int        `json:"port" yaml:"port"`                             // 代理端口
	Reg           *Registry  `json:"reg" yaml:"registry"`                          // 注册中心
	GW            string     `json:"gw" yaml:"gateway"`                            // 网关地址
	GWOk          bool       `json:"gwOk" yaml:"-"`                                // 网关连通状态（运行态）
	DefaultTarget string     `json:"defaultTarget" yaml:"defaultTarget,omitempty"` // 未匹配服务的默认目标：gateway | local（nginx 式兜底）
	DefaultAddr   string     `json:"defaultAddr" yaml:"defaultAddr,omitempty"`     // DefaultTarget=local 时的转发地址（如 127.0.0.1:60018）
	Probe         int        `json:"probe" yaml:"probe,omitempty"`                 // 健康探测间隔（秒，0=默认 5s）
	Services      []*Service `json:"services" yaml:"services"`
	Rules         []*Rule    `json:"rules" yaml:"rules"`
	Running       bool       `json:"running" yaml:"-"`
}

// FindService 按服务名查找
func (e *Environment) FindService(name string) *Service {
	for _, s := range e.Services {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// FindRule 按服务名 + 序号定位规则（前端交互以列表顺序为索引）
func (e *Environment) RulesOf(svc string) []*Rule {
	var out []*Rule
	for _, r := range e.Rules {
		if r.Svc == svc {
			out = append(out, r)
		}
	}
	return out
}

// Global 应用级（不属于任何环境）运行状态
type Global struct {
	Running  bool   `json:"running"`  // 代理是否运行
	LocalIP  string `json:"localIp"`  // 本机局域网 IPv4
	ReqCount int64  `json:"reqCount"` // 请求总数
	AvgMs    int    `json:"avgMs"`    // 滚动平均耗时
	LastSync string `json:"lastSync"` // 最后状态同步时间 HH:MM:SS
	Theme    string `json:"theme"`    // dark | light
}

// LogEntry 一条请求日志（环形队列，上限 50）
// LogEntry 代理请求日志（由回调产出，经事件推送前端）
type LogEntry struct {
	T      string `json:"t"`      // 时间 HH:MM:SS
	M      string `json:"m"`      // 方法 GET/POST
	P      string `json:"p"`      // 请求路径（原始，去前缀前）
	Raw    string `json:"raw"`    // 请求前完整路径（等同 P，语义更明确）
	Final  string `json:"final"`  // 转发后路径（去前缀后；未改写时与 P 相同）
	Target string `json:"target"` // 最终转发目标（网关或本地地址）
	S      string `json:"s"`      // 服务
	Dec    string `json:"dec"`    // 决策：本地 | 网关
	C      int    `json:"c"`      // 状态码
	MS     int    `json:"ms"`     // 耗时 ms
	SName  string `json:"sname"`  // 服务名（冗余，便于前端筛选）
}

// RulePath 规则匹配路径示例；仅作文档辅助
var RulePath = "/order/**"

// Mask 脱敏展示：密码 / Token 在弹窗展示与导出时打码
func Mask(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 4 {
		return "****"
	}
	return v[:2] + strings.Repeat("*", len(v)-2)
}
