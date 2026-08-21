// Package rule 实现路由规则引擎与路径匹配。
//
// 路径模式语法：
//   - /**   匹配任意多层路径（含空）
//   - /*    匹配单层路径
//   - /a/b  字面量精确匹配
package rule

import (
	"strings"

	"voyagergate/internal/model"
)

// MatchPath 判断请求路径是否命中规则模式（自动忽略 query 部分）
func MatchPath(pattern, path string) bool {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	p := splitSegments(pattern)
	s := splitSegments(path)
	return matchSegs(p, s)
}

// splitSegments 将路径拆分为段（忽略首尾空段）
func splitSegments(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// matchSegs 递归匹配段序列，** 匹配 0..n 段
func matchSegs(p, s []string) bool {
	i, j := 0, 0
	for i < len(p) {
		if p[i] == "**" {
			// ** 匹配剩余 0..n 段
			for k := j; k <= len(s); k++ {
				if matchSegs(p[i+1:], s[k:]) {
					return true
				}
			}
			return false
		}
		if j >= len(s) {
			return false
		}
		if p[i] != "*" && p[i] != s[j] {
			return false
		}
		i++
		j++
	}
	return j == len(s)
}

// BestRule 返回路径命中的最高优先级启用规则（prio 数字越小越优先）。
// 若 pattern 相同则按出现顺序取靠前者。
func BestRule(rules []*model.Rule, path string) *model.Rule {
	var best *model.Rule
	for _, r := range rules {
		if r == nil || !r.On {
			continue
		}
		if !MatchPath(r.Path, path) {
			continue
		}
		if best == nil || r.Prio < best.Prio {
			best = r
		}
	}
	return best
}

// StripPrefixPath 去除路径前缀（本地转发场景：本地服务无网关路由前缀）。
// 例：path=/inventory-server/scheduler/x, prefix=/inventory-server → /scheduler/x
// 自动保留 query 部分；前缀不匹配时原样返回。前缀比较忽略大小写。
func StripPrefixPath(path, prefix string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return path
	}
	pathPart := path
	query := ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		pathPart, query = path[:i], path[i:]
	}
	cleaned := strings.TrimPrefix(pathPart, "/")
	lower := strings.ToLower(cleaned)
	lp := strings.ToLower(prefix)
	var out string
	switch {
	case lower == lp:
		out = "/"
	case strings.HasPrefix(lower, lp+"/"):
		out = "/" + strings.TrimPrefix(cleaned[len(prefix):], "/")
	default:
		out = pathPart
	}
	return out + query
}
