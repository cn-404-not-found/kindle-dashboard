package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseNewsFeedRSSAndAtom(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
  <title>频道标题不应被抓取</title>
  <item>
    <title><![CDATA[Linux &amp; RISC-V &mdash; 🚀 新进展]]></title>
    <description><p>正文包含未闭合标签<br>和嵌套元素</p></description>
    <source><title>嵌套源标题应忽略</title></source>
  </item>
  <item><title>券后 99 元包邮某产品大促</title></item>
  <item><title>Go 1.26 发布并发与编译器优化</title></item>
  <item><title>第三条应截断</title></item>
</channel></rss>`

	got, err := parseNewsFeed([]byte(rss), NewsSourceConfig{Key: "ithome", Label: "IT之家", FilterAds: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "Linux & RISC-V — 新进展" || got[1] != "Go 1.26 发布并发与编译器优化" {
		t.Fatalf("unexpected rss titles: %#v", got)
	}

	atom := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>V2EX - 技术</title>
  <entry><title>个人知识库方案对比</title></entry>
  <entry><title>个人知识库方案对比</title></entry>
  <entry><title>软路由自托管服务清单</title></entry>
</feed>`
	got, err = parseNewsFeed([]byte(atom), NewsSourceConfig{Key: "v2ex", Label: "V2EX"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "个人知识库方案对比" || got[1] != "软路由自托管服务清单" {
		t.Fatalf("unexpected atom titles: %#v", got)
	}

	if _, err := parseNewsFeed([]byte(`<!DOCTYPE html><html><body>blocked</body></html>`), NewsSourceConfig{Label: "test"}); err == nil {
		t.Fatal("html challenge page must fail")
	}
}

func TestParseV2EXHotJSON(t *testing.T) {
	body := `[{"title":"🔥 话题一"},{"title":"话题一"},{"title":"话题二"},{"title":"话题三"}]`
	got, err := parseV2EXHotJSON([]byte(body), NewsSourceConfig{Key: "v2ex-hot", Label: "V2EX"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "话题一" || got[1] != "话题二" {
		t.Fatalf("unexpected v2ex json titles: %#v", got)
	}
	if _, err := parseV2EXHotJSON([]byte(`not-json`), NewsSourceConfig{}); err == nil {
		t.Fatal("invalid json must fail")
	}
}

func TestCleanNewsTitleAndAdFilter(t *testing.T) {
	cleaned := cleanNewsTitle("  \t🎉\uFE0F Go\u200D 1.26\n发布 &amp; 更新 ✨ ")
	if cleaned != "Go 1.26 发布 & 更新" {
		t.Fatalf("cleaned = %q", cleaned)
	}
	if !isAdNewsTitle("京东百亿补贴到手价 199 元") || isAdNewsTitle("台积电 2nm 产能爬坡超预期") {
		t.Fatal("unexpected ad filter classification")
	}
}

func TestParseNewsSourcesConfigAndInterval(t *testing.T) {
	defaults := parseNewsSourcesConfig("")
	if len(defaults) != 5 {
		t.Fatalf("default sources = %+v", defaults)
	}
	wantKeys := []string{"solidot", "sspai", "ithome", "geekpark", "v2ex"}
	for i, want := range wantKeys {
		if defaults[i].Key != want {
			t.Fatalf("default[%d] = %q, want %q", i, defaults[i].Key, want)
		}
	}

	custom := parseNewsSourcesConfig("solidot, 我的博客=https://example.com/feed.xml, unknown, bad=ftp://x.test, sspai, ithome, geekpark, v2ex")
	if len(custom) != 5 || custom[0].Key != "solidot" || custom[1].Label != "我的博客" || custom[1].URL != "https://example.com/feed.xml" {
		t.Fatalf("custom = %+v", custom)
	}

	fallback := parseNewsSourcesConfig("unknown-only, bad=not-a-url")
	if len(fallback) != 5 || fallback[0].Key != "solidot" {
		t.Fatalf("invalid config must fall back to defaults: %+v", fallback)
	}

	for raw, want := range map[string]time.Duration{
		"":     30 * time.Minute,
		"600":  10 * time.Minute,
		"120":  30 * time.Minute,
		"abc":  30 * time.Minute,
		"1800": 30 * time.Minute,
	} {
		if got := newsPollIntervalConfig(raw); got != want {
			t.Fatalf("interval(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestTechNewsStoreAndRevision(t *testing.T) {
	var store techNewsStore
	src := NewsSourceConfig{Key: "solidot", Label: "Solidot"}
	store.setSources([]NewsSourceConfig{src})

	loading := store.snapshot()
	store.failSource(src, errors.New("timeout"))
	errored := store.snapshot()
	if techNewsRevision(loading) == techNewsRevision(errored) {
		t.Fatal("transition from loading to all-errored must change revision")
	}

	store.updateSource(src, []string{"标题一", "标题二"}, time.Unix(1000, 0))
	store.failSource(src, errors.New("transient"))
	snap1 := store.snapshot()
	if !snap1.Available || len(snap1.Groups[0].Titles) != 2 || snap1.Groups[0].Err != "transient" {
		t.Fatalf("snap1 = %+v", snap1)
	}
	snap1.Groups[0].Titles[0] = "mutated"
	snap2 := store.snapshot()
	if snap2.Groups[0].Titles[0] != "标题一" {
		t.Fatal("snapshot must deep-copy titles")
	}

	snapNoErr := snap2
	snapNoErr.FetchedAt = time.Unix(2000, 0)
	snapNoErr.Groups = []NewsGroup{{Key: "solidot", Label: "Solidot", Titles: []string{"标题一", "标题二"}}}
	if techNewsRevision(snap2) != techNewsRevision(snapNoErr) {
		t.Fatal("transient error or timestamp with unchanged titles must keep revision")
	}

	snapChanged := snapNoErr
	snapChanged.Groups = []NewsGroup{{Key: "solidot", Label: "Solidot", Titles: []string{"标题三", "标题二"}}}
	if techNewsRevision(snapNoErr) == techNewsRevision(snapChanged) {
		t.Fatal("title change must change revision")
	}
}

func TestFetchNewsSourceHTTPErrors(t *testing.T) {
	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel><item><title>新闻标题 A</title></item></channel></rss>`))
	}))
	defer rssServer.Close()

	titles, err := fetchNewsSource(context.Background(), NewsSourceConfig{Key: "s1", Label: "Test", URL: rssServer.URL, Kind: "feed"})
	if err != nil || len(titles) != 1 || titles[0] != "新闻标题 A" {
		t.Fatalf("titles = %#v, err = %v", titles, err)
	}

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer errServer.Close()
	if _, err := fetchNewsSource(context.Background(), NewsSourceConfig{Key: "s2", Label: "Err", URL: errServer.URL, Kind: "feed"}); err == nil {
		t.Fatal("HTTP 429 must fail")
	}
}

func TestWrapTwoLines(t *testing.T) {
	measure := func(s string) float64 {
		return float64(len([]rune(s))) * 10
	}
	l1, l2 := wrapTwoLines(measure, "短标题", 100, 100)
	if l1 != "短标题" || l2 != "" {
		t.Fatalf("single line got (%q, %q)", l1, l2)
	}

	l1, l2 = wrapTwoLines(measure, "hello world example title", 150, 150)
	if l1 != "hello world" || l2 != "example title" {
		t.Fatalf("word boundary wrap got (%q, %q)", l1, l2)
	}

	l1, l2 = wrapTwoLines(measure, "一二三四五六七八九十一二三四五六七八九十", 80, 80)
	if l1 != "一二三四五六七八" || !strings.HasSuffix(l2, "…") || measure(l2) > 80 {
		t.Fatalf("ellipsis wrap got (%q, %q)", l1, l2)
	}

	// 英文词紧密拼接在中文后时，只沿连续 ASCII 词元回退，不跨越中文误断；
	l1, l2 = wrapTwoLines(measure, "大家觉得Plus订阅真的有必要吗", 60, 200)
	if l1 != "大家觉得" || l2 != "Plus订阅真的有必要吗" {
		t.Fatalf("cjk+ascii word wrap got (%q, %q)", l1, l2)
	}

	// 避头点：避免第二行以句末/分隔标点开头。
	l1, l2 = wrapTwoLines(measure, "一二三四，五六七八", 40, 100)
	if l1 != "一二三" || l2 != "四，五六七八" {
		t.Fatalf("closing punct wrap got (%q, %q)", l1, l2)
	}
}

func TestRenderLandscapeNewsDashboard(t *testing.T) {
	if _, err := loadCalendarFont(fontFile); err != nil {
		t.Skip("Sarasa TTC unavailable")
	}
	now := time.Date(2026, 11, 26, 16, 20, 0, 0, time.FixedZone("UTC+8", 8*3600))
	data := DashboardData{
		TimeStr:          "16:20",
		DateStr:          "2026年11月26日",
		LunarStr:         "十月十八",
		WeatherUpdatedAt: now,
		WeatherAvailable: true,
		Location:         "南宁",
		Temp:             22,
		WeatherText:      "多云",
		HighTemp:         25,
		LowTemp:          18,
		Calendar:         buildMonthGrid(now),
	}
	fullNews := TechNewsSnapshot{
		Available: true,
		FetchedAt: now,
		Groups: []NewsGroup{
			{Key: "solidot", Label: "Solidot", Titles: []string{"Linux 6.18 内核引入更低延迟的内存回收机制与多核任务调度器性能改进", "RISC-V 基金会公布新一代服务器规范，聚焦虚拟化与高密度互联扩展"}},
			{Key: "sspai", Label: "少数派", Titles: []string{"2026 本地知识库与离线全文搜索工具横评：轻量化体积与多端同步谁更值得选", "从零打造极简墨水屏桌面效率看板：字体排版与长期稳定运行经验总结"}},
			{Key: "ithome", Label: "IT之家", Titles: []string{"台积电 2nm 先进制程产能爬坡超预期，多款旗舰移动与端侧 AI 芯片进入量产期", "Go 1.26 路线图预览：标准库并发原语持续升级，构建速度双优化"}},
			{Key: "geekpark", Label: "极客公园", Titles: []string{"AI 编程助手迈入多智能体协作阶段，企业代码审查工作流正在全面重构", "端侧大模型本地推理成本半年再降四成，轻量模型落地节奏明显提速"}},
			{Key: "v2ex", Label: "V2EX", Titles: []string{"大家现在家里软路由和 NAS 上都跑了哪些长期稳定不折腾的自托管服务？", "分享一套零外部依赖、纯 Go 编写的极简家庭墨水屏信息中心架构实践"}},
		},
	}
	for _, tc := range []struct {
		name string
		snap TechNewsSnapshot
	}{
		{"five sources", fullNews},
		{"empty loading", TechNewsSnapshot{Groups: []NewsGroup{{Key: "solidot", Label: "Solidot"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pngBytes, err := renderLandscapeNewsDashboard(data, tc.snap)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(pngBytes))
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != image.Rect(0, 0, 1448, 1072) {
				t.Fatalf("bounds = %v", img.Bounds())
			}
			black := 0
			for y := 0; y < img.Bounds().Dy(); y += 7 {
				for x := 690; x < img.Bounds().Dx(); x += 7 {
					if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
						black++
					}
				}
			}
			if black == 0 {
				t.Fatal("right column is blank")
			}
		})
	}
}

func TestTechNewsHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/news", nil)
	rec := httptest.NewRecorder()
	techNewsHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var snap TechNewsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/news", nil)
	postRec := httptest.NewRecorder()
	techNewsHandler(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("post status = %d", postRec.Code)
	}
}
