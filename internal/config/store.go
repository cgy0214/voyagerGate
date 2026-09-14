// Package config 负责配置文件的加载与持久化（~/.voyagerGate/config.yaml）。
package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"voyagergate/internal/model"
	"voyagergate/internal/version"
)

var Version = version.Display()

const (
	// MaxEnvs 环境数量上限（防御性限制）
	MaxEnvs = 50
)

// File 配置文件磁盘结构
type File struct {
	Version   string               `yaml:"version"`
	Theme     string               `yaml:"theme"`
	Lang      string               `yaml:"lang"`      // zh | en，UI 语言
	LocalOnly bool                 `yaml:"localOnly"` // true = 仅绑 127.0.0.1；false/缺省 = 绑 0.0.0.0（历史行为）
	Current   string               `yaml:"current"`
	Envs      []*model.Environment `yaml:"environments"`
}

type Store struct {
	path string
}

// NewStore 创建配置仓库（懒加载，不在此处读写）
func NewStore() *Store {
	return &Store{path: ConfigPath()}
}

// NewStoreAt 使用指定配置文件路径创建仓库（测试用）
func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".voyagerGate", "config.yaml")
}

// ConfigDir 返回配置目录 ~/.voyagerGate
func ConfigDir() string {
	return filepath.Dir(ConfigPath())
}

// Load 加载配置文件；文件不存在或损坏时回退到空白配置（不报错打断启动）。
func (s *Store) Load() *File {
	// LocalOnly 零值 false = 绑 0.0.0.0，与历史版本行为一致；设置中开启"仅本机"后写 true
	f := &File{Version: Version, Theme: "dark"}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return f
	}
	var disk File
	if err := yaml.Unmarshal(data, &disk); err != nil {
		return f
	}
	if disk.Version == "" {
		disk.Version = Version
	}
	if disk.Theme == "" {
		disk.Theme = "dark"
	}
	if disk.Lang == "" {
		disk.Lang = "zh"
	}
	disk.Envs = sanitize(disk.Envs)
	// 迁移：早期自动 /** 兜底规则优先级为 1，会压过用户具体规则 → 统一降为最低优先级
	for _, e := range disk.Envs {
		for _, r := range e.Rules {
			if r != nil && r.Path == "/**" && r.Prio < 99 {
				r.Prio = 99
			}
		}
	}
	return &disk
}

// Save 将配置原子写入磁盘（先写临时文件再改名，避免写一半损坏）。
func (s *Store) Save(f *File) error {
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	if f.Version == "" {
		f.Version = Version
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// sanitize 过滤空环境、补全缺失字段
func sanitize(envs []*model.Environment) []*model.Environment {
	var out []*model.Environment
	for _, e := range envs {
		if e == nil || e.Name == "" {
			continue
		}
		if e.Reg == nil {
			e.Reg = &model.Registry{Type: model.TypeNacos, Addr: "", NS: "", Group: "DEFAULT_GROUP", NacosVer: "1"}
		}
		if e.Reg.Type == "" {
			e.Reg.Type = model.TypeNacos
		}
		if e.Reg.NacosVer == "" {
			e.Reg.NacosVer = "1"
		}
		if e.Services == nil {
			e.Services = []*model.Service{}
		}
		if e.Rules == nil {
			e.Rules = []*model.Rule{}
		}
		out = append(out, e)
	}
	return out
}

// DefaultEnv 生成一个全新的空白环境（注册中心默认 Nacos + dev 命名空间，与需求一致）。
func DefaultEnv(name string) *model.Environment {
	return &model.Environment{
		Name: name,
		Port: 6000,
		Reg: &model.Registry{
			Type:     model.TypeNacos,
			Addr:     "",
			NS:       "",
			Group:    "",
			NacosVer: "1",
		},
		GW:       "",
		GWOk:     false,
		Services: []*model.Service{},
		Rules:    []*model.Rule{},
	}
}

// PreviewYAML 生成当前环境的 YAML 预览文本（密码/Token 脱敏），供配置弹窗展示。
func PreviewYAML(e *model.Environment, localIP string) string {
	reg := e.Reg
	if reg == nil {
		reg = &model.Registry{}
	}
	line := "  addr: " + reg.Addr
	switch reg.Type {
	case model.TypeNacos:
		if reg.NacosVer != "" {
			line += "\n  nacosVersion: " + reg.NacosVer
		}
		if reg.NS != "" {
			line += "\n  namespace: " + reg.NS
		}
		if reg.Group != "" {
			line += "\n  group: " + reg.Group
		}
		if reg.NacosUser != "" {
			line += "\n  nacosUsername: " + reg.NacosUser
		}
		if reg.NacosPass != "" {
			line += "\n  nacosPassword: " + model.Mask(reg.NacosPass)
		}
	case model.TypeEureka:
		if reg.User != "" {
			line += "\n  username: " + reg.User
		}
		if reg.Pass != "" {
			line += "\n  password: " + model.Mask(reg.Pass)
		}
	case model.TypeConsul:
		if reg.DC != "" {
			line += "\n  datacenter: " + reg.DC
		}
		if reg.Token != "" {
			line += "\n  token: " + model.Mask(reg.Token)
		}
	}
	out := "# VoyagerGate 渡桥 配置\n"
	out += "version: " + Version + "\n\n"
	out += "environment:\n  name: " + e.Name + "\n  port: " + strconv.Itoa(e.Port) + "\n  localIp: " + localIP + "\n\n"
	out += "registry:\n  type: " + reg.Type + "\n" + line + "\n\n"
	out += "gateway: " + e.GW + "\n\n"
	out += "services:\n"
	if len(e.Services) == 0 {
		out += "  []\n"
	}
	for _, s := range e.Services {
		src := "registry"
		if s.Src == model.SrcManual {
			src = "manual"
		}
		out += "  - name: " + s.Name + "\n    source: " + src + "\n    enabled: " + boolStr(s.On) + "\n    port: " + strconv.Itoa(s.Port) + "\n"
		if s.Prefix != "" {
			out += "    prefix: " + s.Prefix + "\n"
		}
	}
	out += "\nrules:\n"
	if len(e.Rules) == 0 {
		out += "  []\n"
	}
	for _, r := range e.Rules {
		out += "  - service: " + r.Svc + "\n    path: " + r.Path + "\n    dest: " + r.Dest + "\n    priority: " + strconv.Itoa(r.Prio) + "\n    enabled: " + boolStr(r.On) + "\n"
		if r.StripPrefix != "" {
			out += "    stripPrefix: " + r.StripPrefix + "\n"
		}
		if r.Remark != "" {
			out += "    remark: " + r.Remark + "\n"
		}
	}
	return out
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
