// Package proxy 实现服务健康探测。
package proxy

import (
	"math/rand"
	"net"
	"time"
)

// probeTimeout 本地服务 TCP 探测超时
const probeTimeout = 800 * time.Millisecond

// ProbeTCP 通过 TCP 拨号探测本地服务是否在线
func ProbeTCP(addr string) bool {
	if addr == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", addr, probeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// MaybeFlake 以极小概率随机翻转探测结果，模拟本地服务偶发抖动。
// 用于无本地目标地址（无法真实探测）的服务，贴合需求文档「小概率随机变化」。
func MaybeFlake(online bool) bool {
	if rand.Float32() < 0.08 {
		return !online
	}
	return online
}