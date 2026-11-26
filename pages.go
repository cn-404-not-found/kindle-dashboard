// pages.go —— 横屏看板的页面注册与分发
//
// 页面顺序由服务端维护：客户端只保存当前页名，并通过响应头里的
// X-Dashboard-Next-Page / X-Dashboard-Prev-Page 翻页，这样以后加页不需要改客户端。
package main

import "time"

const (
	dashboardPageHome       = "home"
	dashboardPageOpencode   = "opencode"
	dashboardPageCalendar   = "calendar"
	dashboardPageNewsDetail = "news_detail"
)

// dashboardPageNames 是横屏客户端的页面顺序。
var dashboardPageNames = []string{dashboardPageHome, dashboardPageOpencode, dashboardPageCalendar}

// dashboardPage 描述一次请求对应的页面位置与相邻页。
type dashboardPage struct {
	Name  string
	Index int // 1-based
	Total int
	Prev  string
	Next  string
}

// resolveDashboardPage 解析 page 参数；空值回退首页，未知页面返回 false。
// news_detail 为按需点击进入的独立阅读页，不参与日常 3 页滑动轮换。
func resolveDashboardPage(requested string) (dashboardPage, bool) {
	if requested == "" {
		requested = dashboardPageHome
	}
	if requested == dashboardPageNewsDetail {
		return dashboardPage{
			Name:  dashboardPageNewsDetail,
			Index: 1,
			Total: 1,
			Prev:  dashboardPageHome,
			Next:  dashboardPageHome,
		}, true
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
	case dashboardPageCalendar:
		return renderLandscapeDashboard(data)
	default:
		return renderLandscapeNewsDashboard(data, techNews.snapshot())
	}
}

// dashboardPageSupportsPartial 表示该页面的指定区域支持局刷。
// 热榜首页支持时钟/天气/右侧热榜局刷；完整月历页支持时钟/天气；OpenCode 页支持页头局刷。
func dashboardPageSupportsPartial(name, part string) bool {
	switch name {
	case dashboardPageHome:
		return part == "clock" || part == "weather" || part == "news"
	case dashboardPageCalendar:
		return part == "clock" || part == "weather"
	case dashboardPageOpencode:
		return part == "pageheader"
	default:
		return false
	}
}
