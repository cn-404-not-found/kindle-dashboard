// pages.go —— 横屏看板的页面注册与分发
//
// 页面顺序由服务端维护：客户端只保存当前页名，并通过响应头里的
// X-Dashboard-Next-Page / X-Dashboard-Prev-Page 翻页，这样以后加页不需要改客户端。
package main

import "time"

const (
	dashboardPageHome     = "home"
	dashboardPageOpencode = "opencode"
)

// dashboardPageNames 是横屏客户端的页面顺序。
var dashboardPageNames = []string{dashboardPageHome, dashboardPageOpencode}

// dashboardPage 描述一次请求对应的页面位置与相邻页。
type dashboardPage struct {
	Name  string
	Index int // 1-based
	Total int
	Prev  string
	Next  string
}

// resolveDashboardPage 解析 page 参数；空值回退首页，未知页面返回 false。
func resolveDashboardPage(requested string) (dashboardPage, bool) {
	if requested == "" {
		requested = dashboardPageHome
	}
	for i, name := range dashboardPageNames {
		if name != requested {
			continue
		}
		total := len(dashboardPageNames)
		return dashboardPage{
			Name:  name,
			Index: i + 1,
			Total: total,
			Prev:  dashboardPageNames[(i-1+total)%total],
			Next:  dashboardPageNames[(i+1)%total],
		}, true
	}
	return dashboardPage{}, false
}

// renderDashboardPage 按页面名分发渲染。
func renderDashboardPage(page dashboardPage, data DashboardData, now time.Time) ([]byte, error) {
	switch page.Name {
	case dashboardPageOpencode:
		return renderOpencodeDashboard(opencodeUsage.snapshot(), now, page.Index, page.Total)
	default:
		return renderLandscapeDashboard(data)
	}
}

// dashboardPageSupportsPartial 表示该页面的指定区域支持局刷。
// 首页有时钟/天气两块动态区域；OpenCode 页的页头可单独更新，正文按需整页刷新。
func dashboardPageSupportsPartial(name, part string) bool {
	switch name {
	case dashboardPageHome:
		return part == "clock" || part == "weather"
	case dashboardPageOpencode:
		return part == "pageheader"
	default:
		return false
	}
}
