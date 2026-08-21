// Package registry 实现注册中心统一抽象层。
package registry

import (
	"fmt"

	"voyagergate/internal/model"
)

// Adapter 注册中心适配器统一接口
type Adapter interface {
	// Type 注册中心类型（Nacos / Eureka / Consul）
	Type() string
	// Test 连通性测试（连接成功返回 nil）
	Test() error
	// FetchServices 拉取服务列表，返回服务名 + 首个实例端口（本地开发端口假定与注册端口一致）
	FetchServices() ([]*model.Service, error)
}

// New 依据注册中心配置创建对应适配器
func New(reg *model.Registry) (Adapter, error) {
	if reg == nil {
		return nil, fmt.Errorf("注册中心配置为空")
	}
	switch reg.Type {
	case model.TypeNacos:
		return newNacos(reg), nil
	case model.TypeEureka:
		return newEureka(reg), nil
	case model.TypeConsul:
		return newConsul(reg), nil
	default:
		return nil, fmt.Errorf("暂不支持的注册中心类型: %s（本期支持 Nacos / Eureka / Consul）", reg.Type)
	}
}

// normalizeAddr 为裸地址补充协议与默认端口，保证可发起 HTTP 请求。
func normalizeAddr(addr string, defPort int) string {
	if addr == "" {
		return ""
	}
	hasScheme := len(addr) > 7 && (addr[:7] == "http://" || addr[:8] == "https://")
	if !hasScheme {
		addr = "http://" + addr
	}
	return addr
}
