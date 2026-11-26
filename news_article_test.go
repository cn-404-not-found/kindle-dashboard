package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseNewsFeedItemsAndV2EXItems(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel>
  <title>频道标题</title>
  <item>
    <title>RSS 完整正文条目</title>
    <link>https://example.com/posts/100</link>
    <pubDate>Thu, 26 Nov 2026 10:30:00 +0800</pubDate>
    <description><p>短摘要...</p></description>
    <content:encoded><![CDATA[<p>第一段完整正文，介绍架构细节。</p><p>第二段补充基准测试结果与结论。</p>]]></content:encoded>
    <source><title>嵌套源应忽略</title></source>
  </item>
  <item>
    <title>少数派风格截断摘要条目</title>
    <link>https://example.com/posts/101</link>
    <pubDate>2026-11-26T08:00:00+08:00</pubDate>
    <description><![CDATA[<p>这是导语部分的内容。查看全文</p>]]></description>
  </item>
  <item>
    <title>券后 99 元包邮某商品</title>
    <link>https://example.com/ad</link>
  </item>
</channel></rss>`

	items, err := parseNewsFeedItems([]byte(rss), NewsSourceConfig{Key: "ithome", Label: "IT之家", FilterAds: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %#v", len(items), items)
	}
	if items[0].ID == "" || items[0].PublishedAt != "2026-11-26" || items[0].Link != "https://example.com/posts/100" {
		t.Fatalf("item[0] metadata unexpected: %+v", items[0])
	}
	if !strings.Contains(items[0].FeedBody, "第一段完整正文") || !strings.Contains(items[0].FeedBody, "\n\n第二段补充") || items[0].FeedTruncated {
		t.Fatalf("item[0] body unexpected: %+v", items[0])
	}
	if !items[1].FeedTruncated || items[1].FeedBody != "这是导语部分的内容。" {
		t.Fatalf("item[1] truncated detection failed: %+v", items[1])
	}

	atom := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <title>Atom 正文测试</title>
    <link rel="replies" href="https://example.com/replies" />
    <link rel="alternate" href="https://example.com/t/42" />
    <published>2026-11-26T09:15:00+08:00</published>
    <content type="html">&lt;p&gt;Atom 内容段落一。&lt;/p&gt;&lt;p&gt;Atom 内容段落二。&lt;/p&gt;</content>
  </entry>
</feed>`
	atomItems, err := parseNewsFeedItems([]byte(atom), NewsSourceConfig{Key: "v2ex", Label: "V2EX"})
	if err != nil {
		t.Fatal(err)
	}
	if len(atomItems) != 1 || atomItems[0].Link != "https://example.com/t/42" || atomItems[0].PublishedAt != "2026-11-26" {
		t.Fatalf("atom item unexpected: %+v", atomItems)
	}
	if !strings.Contains(atomItems[0].FeedBody, "Atom 内容段落二。") {
		t.Fatalf("atom body = %q", atomItems[0].FeedBody)
	}

	v2exBody := `[{"title":"V2EX 热帖","url":"https://www.v2ex.com/t/99","content":"这是 JSON 正文\n\n第二段。","created":1795665600}]`
	v2Items, err := parseV2EXHotJSONItems([]byte(v2exBody), NewsSourceConfig{Key: "v2ex-hot", Label: "V2EX"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v2Items) != 1 || v2Items[0].Link != "https://www.v2ex.com/t/99" || !strings.Contains(v2Items[0].FeedBody, "这是 JSON 正文") {
		t.Fatalf("v2ex items = %+v", v2Items)
	}
}

func TestExtractWebArticleTextAndResolveContent(t *testing.T) {
	sampleHTML := `<!doctype html><html><body>
	<header><p>顶部导航不应该出现</p></header>
	<div class="article-wrap">
		<div class="content wangEditor-txt">
			<div class="inner-box"><p>这是嵌套 div 里的少数派正文第一段，内容充实完整，方便测试容器边界识别。</p></div>
			<p>正文第二段包含更多技术细节，确保字数达到正文提取阈值并保留段落结构。</p>
		</div>
	</div>
	<footer><p>页脚版权声明不应进入正文容器。</p></footer>
	</body></html>`

	extracted := extractWebArticleText([]byte(sampleHTML))
	if !strings.Contains(extracted, "这是嵌套 div 里的少数派正文第一段") || !strings.Contains(extracted, "正文第二段包含更多技术细节") {
		t.Fatalf("unexpected extracted body: %q", extracted)
	}
	if strings.Contains(extracted, "页脚版权声明") || strings.Contains(extracted, "顶部导航") {
		t.Fatalf("extracted body leaked layout chrome: %q", extracted)
	}

	// <p> 回退提取测试
	fallbackHTML := `<html><body><nav><p>导航短文字</p></nav><p>这是一篇没有标准容器类名的文章的第一自然段正文内容，长度足以通过阈值检查。</p><p>第二自然段补充分析。</p></body></html>`
	fb := extractWebArticleText([]byte(fallbackHTML))
	if !strings.Contains(fb, "第一自然段正文内容") || strings.Contains(fb, "导航短文字") {
		t.Fatalf("unexpected fallback body: %q", fb)
	}

	// 1. RSS 已完整时不发 HTTP 请求
	fullText := strings.Repeat("完整正文。", 50)
	body, isFull := resolveArticleContent(context.Background(), NewsItem{
		ID:       "test-rss-full-1",
		Link:     "http://127.0.0.1:1/should-not-request",
		FeedBody: fullText,
	})
	if !isFull || body != fullText {
		t.Fatalf("RSS full item got isFull=%v len=%d", isFull, len(body))
	}

	// 2. 截断条目按需抓网页并缓存，关闭测试服务器后二次读取仍命中缓存
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(sampleHTML))
	}))
	item := NewsItem{
		ID:            "test-web-fetch-1",
		Link:          srv.URL,
		FeedBody:      "短短一句导语",
		FeedTruncated: true,
	}
	gotBody, gotFull := resolveArticleContent(context.Background(), item)
	srv.Close()
	if !gotFull || hits != 1 || !strings.Contains(gotBody, "这是嵌套 div 里的少数派正文第一段") {
		t.Fatalf("web fetch got isFull=%v hits=%d body=%q", gotFull, hits, gotBody)
	}
	cachedBody, cachedFull := resolveArticleContent(context.Background(), item)
	if !cachedFull || cachedBody != gotBody {
		t.Fatalf("cache hit failed: isFull=%v body=%q", cachedFull, cachedBody)
	}

	// 3. 抓取失败时降级返回 RSS 导语且标记为非完整全文
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errSrv.Close()
	fallbackBody, fallbackFull := resolveArticleContent(context.Background(), NewsItem{
		ID:            "test-web-fail-1",
		Link:          errSrv.URL,
		FeedBody:      "仅存导语摘要",
		FeedTruncated: true,
	})
	if fallbackFull || fallbackBody != "仅存导语摘要" {
		t.Fatalf("failure fallback got isFull=%v body=%q", fallbackFull, fallbackBody)
	}
}

func TestRawTouchToLandscapePW2AndPW3(t *testing.T) {
	if _, _, ok := rawTouchToLandscape(100, 100, 0, 1024); ok {
		t.Fatal("zero framebuffer dimensions must fail")
	}
	if _, _, ok := rawTouchToLandscape(-1, 10, 758, 1024); ok {
		t.Fatal("negative coordinate must fail")
	}

	for _, tc := range []struct {
		name     string
		fbW, fbH int
	}{
		{"PW2", 758, 1024},
		{"PW3", 1072, 1448},
	} {
		// Center tap maps near (724, 536).
		lx, ly, ok := rawTouchToLandscape(tc.fbW/2, tc.fbH/2, tc.fbW, tc.fbH)
		if !ok || lx < 715 || lx > 735 || ly < 525 || ly > 545 {
			t.Fatalf("%s center got (%v, %v) ok=%v", tc.name, lx, ly, ok)
		}

		// Landscape top-left back button center (lx=163, ly=62).
		rawX := int((1071.0 - 62.0) * float64(tc.fbW) / 1072.0)
		rawY := int(163.0 * float64(tc.fbH) / 1448.0)
		bx, by, ok := rawTouchToLandscape(rawX, rawY, tc.fbW, tc.fbH)
		if !ok || bx > 480 || by > 125 {
			t.Fatalf("%s top-left back button mapped to (%v, %v) ok=%v", tc.name, bx, by, ok)
		}
		if got := hitTestArticleDetail(bx, by, "id1", 2, 5); got != "home" {
			t.Fatalf("%s top-left action = %q, want home", tc.name, got)
		}
	}
}

func TestHitTestHomeNewsAndArticleDetail(t *testing.T) {
	snap := TechNewsSnapshot{
		Available: true,
		Groups: []NewsGroup{
			{Key: "g1", Label: "Solidot", Titles: []string{"t0", "t1"}, Items: []NewsItem{{ID: "id0"}, {ID: "id1"}}},
			{Key: "g2", Label: "少数派", Titles: []string{"t2", "t3"}, Items: []NewsItem{{ID: "id2"}, {ID: "id3"}}},
			{Key: "g3", Label: "IT之家", Titles: []string{"t4", "t5"}, Items: []NewsItem{{ID: "id4"}, {ID: "id5"}}},
			{Key: "g4", Label: "极客公园", Titles: []string{"t6", "t7"}, Items: []NewsItem{{ID: "id6"}, {ID: "id7"}}},
			{Key: "g5", Label: "V2EX", Titles: []string{"t8", "t9"}, Items: []NewsItem{{ID: "id8"}, {ID: "id9"}}},
		},
	}

	if _, hit := hitTestHomeNews(300, 400, snap); hit {
		t.Fatal("left column tap must not hit news")
	}

	// Test every single one of all 10 slots mapped from PW2 (758x1024).
	for slot := 0; slot < 10; slot++ {
		groupIdx := slot / 2
		itemIdx := slot % 2
		blockY := newsTopY + float64(groupIdx)*newsBlockStep
		targetLY := blockY + 45
		if itemIdx == 1 {
			targetLY = blockY + 135
		}
		targetLX := 980.0
		rawX := int((1071.0 - targetLY) * 758.0 / 1072.0)
		rawY := int(targetLX * 1024.0 / 1448.0)
		lx, ly, ok := rawTouchToLandscape(rawX, rawY, 758, 1024)
		if !ok {
			t.Fatalf("slot %d rawTouchToLandscape failed", slot)
		}
		item, hit := hitTestHomeNews(lx, ly, snap)
		wantID := snap.Groups[groupIdx].Items[itemIdx].ID
		if !hit || item.ID != wantID {
			t.Fatalf("slot %d (lx=%.1f, ly=%.1f) hit=%v id=%q, want %q", slot, lx, ly, hit, item.ID, wantID)
		}
	}

	if got := hitTestArticleDetail(150, 1020, "a1", 1, 3); got != "noop" {
		t.Fatalf("prev on page 1 got %q, want noop", got)
	}
	if got := hitTestArticleDetail(150, 1020, "a1", 2, 3); got != "open a1 1" {
		t.Fatalf("prev on page 2 got %q, want open a1 1", got)
	}
	if got := hitTestArticleDetail(1300, 1020, "a1", 2, 3); got != "open a1 3" {
		t.Fatalf("next on page 2 got %q, want open a1 3", got)
	}
	if got := hitTestArticleDetail(1300, 1020, "a1", 3, 3); got != "noop" {
		t.Fatalf("next on last page got %q, want noop", got)
	}
	if got := hitTestArticleDetail(724, 536, "a1", 2, 3); got != "noop" {
		t.Fatalf("center reading tap got %q, want noop", got)
	}
}

func TestTapHandlerAndNewsDetailScreenHandler(t *testing.T) {
	src := NewsSourceConfig{Key: "geekpark", Label: "极客公园"}
	bodyText := strings.Repeat("这是一段用于测试完整正文阅读与分页渲染的中文科技文章内容。\n\n", 18)
	item := NewsItem{
		ID:          "abc123def456",
		SourceKey:   src.Key,
		SourceLabel: src.Label,
		Title:       "点击后全屏阅读与翻页测试标题",
		PublishedAt: "2026-11-26",
		FeedBody:    bodyText,
	}
	techNews.setSources([]NewsSourceConfig{src})
	defer techNews.setSources(nil)
	techNews.updateSourceItems(src, []NewsItem{item}, time.Now())

	// PW2 coordinates corresponding to landscape (lx=980, ly=95) -> first item.
	lx, ly := 980.0, 95.0
	rawX := int((1071.0 - ly) * 758.0 / 1072.0)
	rawY := int(lx * 1024.0 / 1448.0)
	req := httptest.NewRequest(http.MethodGet, "/api/tap?page=home&w=758&h=1024&x="+itoa(rawX)+"&y="+itoa(rawY), nil)
	rec := httptest.NewRecorder()
	tapHandler(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "open abc123def456 1" {
		t.Fatalf("tap home got code=%d body=%q", rec.Code, rec.Body.String())
	}

	leftReq := httptest.NewRequest(http.MethodGet, "/api/tap?page=home&w=758&h=1024&x=400&y=200", nil)
	leftRec := httptest.NewRecorder()
	tapHandler(leftRec, leftReq)
	if strings.TrimSpace(leftRec.Body.String()) != "noop" {
		t.Fatalf("tap left column = %q, want noop", leftRec.Body.String())
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/tap", nil)
	postRec := httptest.NewRecorder()
	tapHandler(postRec, postReq)
	if postRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/tap status = %d", postRec.Code)
	}

	if _, err := loadCalendarFont(fontFile); err != nil {
		t.Skip("Sarasa TTC unavailable")
	}
	appLoc = time.FixedZone("UTC+8", 8*3600)

	screenReq := httptest.NewRequest(http.MethodGet, "/screen.png?page=news_detail&id=abc123def456&p=2", nil)
	screenReq.Header.Set("X-Dashboard-Layout", "landscape-v1")
	screenReq.Header.Set("png-width", "758")
	screenReq.Header.Set("png-height", "1024")
	screenRec := httptest.NewRecorder()
	screenHandler(screenRec, screenReq)
	if screenRec.Code != http.StatusOK {
		t.Fatalf("screenHandler status = %d body=%s", screenRec.Code, screenRec.Body.String())
	}
	if screenRec.Header().Get("X-Dashboard-Page") != "news_detail" ||
		screenRec.Header().Get("X-Dashboard-Article-Id") != "abc123def456" ||
		screenRec.Header().Get("X-Dashboard-Article-Page") != "2" {
		t.Fatalf("unexpected headers: %#v", screenRec.Header())
	}
	img, err := png.Decode(bytes.NewReader(screenRec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, 758, 1024) {
		t.Fatalf("PW2 bounds = %v, want 758x1024", img.Bounds())
	}

	// Unknown / expired ID returns clean fallback notice page rather than 500.
	missReq := httptest.NewRequest(http.MethodGet, "/screen.png?page=news_detail&id=missingid", nil)
	missReq.Header.Set("X-Dashboard-Layout", "landscape-v1")
	missRec := httptest.NewRecorder()
	screenHandler(missRec, missReq)
	if missRec.Code != http.StatusOK || missRec.Header().Get("X-Dashboard-Article-Total") != "1" {
		t.Fatalf("missing ID status=%d headers=%v", missRec.Code, missRec.Header())
	}
}

func TestWrapParagraphPaginateAndRenderArticle(t *testing.T) {
	measure := func(s string) float64 {
		return float64(len([]rune(s))) * 10
	}
	wrapped := wrapParagraph(measure, "第一句中文测试PlusSubscription功能，继续补齐后面句子内容，确保正确换行。", 120)
	if len(wrapped) < 2 {
		t.Fatalf("expected multi-line wrap, got %v", wrapped)
	}
	for _, line := range wrapped {
		if strings.HasPrefix(line, "，") || strings.HasPrefix(line, "。") {
			t.Fatalf("line starts with closing punctuation: %q", line)
		}
	}

	lines := buildArticleLines(measure, strings.Repeat("段落一行内容。\n\n", 20), 200)
	pages := paginateArticle(lines, 240, 360, 60, 24)
	if len(pages) < 2 {
		t.Fatalf("expected >= 2 pages, got %d", len(pages))
	}
	if emptyPages := paginateArticle(nil, 240, 360, 60, 24); len(emptyPages) != 1 {
		t.Fatalf("empty pagination = %d pages, want 1", len(emptyPages))
	}

	if _, err := loadCalendarFont(fontFile); err != nil {
		t.Skip("Sarasa TTC unavailable")
	}
	item := NewsItem{
		ID:          "render-test-1",
		SourceLabel: "少数派",
		Title:       "这是一个横屏全屏阅读页面的标题渲染测试",
		PublishedAt: "2026-11-26",
	}
	longBody := strings.Repeat("这是全屏阅读正文的一整段完整示例文本，用来测试三十八像素正文的换行效果、行距与多页切换显示。\n\n", 14)
	p1, num1, total, err := renderNewsArticlePage(item, longBody, true, 1)
	if err != nil {
		t.Fatal(err)
	}
	if num1 != 1 || total < 2 {
		t.Fatalf("p1 metadata num=%d total=%d", num1, total)
	}
	pLast, numLast, totalLast, err := renderNewsArticlePage(item, longBody, true, 999)
	if err != nil || numLast != totalLast || totalLast != total {
		t.Fatalf("clamped last page num=%d total=%d err=%v", numLast, totalLast, err)
	}
	for _, raw := range [][]byte{p1, pLast} {
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds() != image.Rect(0, 0, 1448, 1072) {
			t.Fatalf("bounds = %v", img.Bounds())
		}
		black := 0
		for y := 0; y < img.Bounds().Dy(); y += 9 {
			for x := 0; x < img.Bounds().Dx(); x += 9 {
				if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
					black++
				}
			}
		}
		if black == 0 {
			t.Fatal("rendered article page is blank")
		}
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
