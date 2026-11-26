// tech_news.go —— 中文科技热榜 RSS/Atom/JSON 采集与缓存
//
// 定时从预设或自定义中文科技源（Solidot、少数派、IT之家、极客公园、V2EX 技术区）
// 并发抓取最新条目，每个源保留 2 条标题供横屏首页右侧展示。
//
// 设计要点：
//   - 默认每 30 分钟后台抓取一次，单个源失败时保留该源上次成功数据；
//   - 自动过滤 IT之家 等快讯源中的导购促销标题，并清除 Emoji 防止墨水屏出现方框；
//   - 仅以展示标题内容计算 Revision 哈希，标题未变化时不触发 Kindle 右侧局刷。
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	newsDefaultInterval    = 30 * time.Minute
	newsMinInterval        = 5 * time.Minute
	newsDefaultHTTPTimeout = 12 * time.Second
	newsMaxResponseBytes   = 2 << 20 // 2 MiB，兼容部分含全文摘要的 RSS
	newsMaxSources         = 5
	newsItemsPerSource     = 2
	newsDefaultSourcesSpec = "solidot,sspai,ithome,geekpark,v2ex"
)

// NewsSourceConfig 描述一个待采集的科技资讯源。
type NewsSourceConfig struct {
	Key       string
	Label     string
	URL       string
	Kind      string // "feed" (RSS/Atom) 或 "v2ex_json"
	FilterAds bool
}

// NewsItem 表示单条科技资讯及其标题、来源、链接、发布日期和 RSS 自带正文。
type NewsItem struct {
	ID            string `json:"id"`
	SourceKey     string `json:"source_key"`
	SourceLabel   string `json:"source_label"`
	Title         string `json:"title"`
	Link          string `json:"link,omitempty"`
	PublishedAt   string `json:"published_at,omitempty"`
	FeedBody      string `json:"-"`
	FeedTruncated bool   `json:"-"`
}

// NewsGroup 表示单个来源分组的采集结果，供 JSON 接口和页面渲染使用。
type NewsGroup struct {
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Titles    []string   `json:"titles"`
	Items     []NewsItem `json:"items,omitempty"`
	FetchedAt time.Time  `json:"fetched_at,omitempty"`
	Err       string     `json:"error,omitempty"`
}

// TechNewsSnapshot 是横屏首页右侧展示与 /api/news 响应使用的快照。
type TechNewsSnapshot struct {
	Available bool        `json:"available"`
	FetchedAt time.Time   `json:"fetched_at"`
	Groups    []NewsGroup `json:"groups"`
}

var newsSourcePresets = map[string]NewsSourceConfig{
	"solidot": {
		Key:   "solidot",
		Label: "Solidot",
		URL:   "https://www.solidot.org/index.rss",
		Kind:  "feed",
	},
	"sspai": {
		Key:   "sspai",
		Label: "少数派",
		URL:   "https://sspai.com/feed",
		Kind:  "feed",
	},
	"ithome": {
		Key:       "ithome",
		Label:     "IT之家",
		URL:       "https://www.ithome.com/rss/",
		Kind:      "feed",
		FilterAds: true,
	},
	"geekpark": {
		Key:   "geekpark",
		Label: "极客公园",
		URL:   "https://www.geekpark.net/rss",
		Kind:  "feed",
	},
	"v2ex": {
		Key:   "v2ex",
		Label: "V2EX",
		URL:   "https://www.v2ex.com/feed/tab/tech.xml",
		Kind:  "feed",
	},
	"v2ex-hot": {
		Key:   "v2ex-hot",
		Label: "V2EX",
		URL:   "https://www.v2ex.com/api/topics/hot.json",
		Kind:  "v2ex_json",
	},
	"ruanyifeng": {
		Key:   "ruanyifeng",
		Label: "阮一峰",
		URL:   "https://www.ruanyifeng.com/blog/atom.xml",
		Kind:  "feed",
	},
	"leiphone": {
		Key:   "leiphone",
		Label: "雷峰网",
		URL:   "https://www.leiphone.com/feed",
		Kind:  "feed",
	},
}

var newsAdKeywords = []string{
	"券后", "到手价", "立减", "满减", "领券", "国补", "补贴价",
	"直降", "史低", "好价", "大促", "秒杀", "包邮", "百亿补贴",
	"天猫超市", "下单立减", "折合",
}

const newsMaxItemHistory = 50

// techNewsStore 保护后台抓取线程与 HTTP 渲染之间的并发读写。
type techNewsStore struct {
	mu          sync.RWMutex
	sources     []NewsSourceConfig
	byKey       map[string]NewsGroup
	itemHistory map[string]NewsItem
	itemOrder   []string
	fetchedAt   time.Time
}

var techNews techNewsStore

func (s *techNewsStore) setSources(sources []NewsSourceConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources = append([]NewsSourceConfig(nil), sources...)
	s.byKey = make(map[string]NewsGroup, len(sources))
	for _, src := range sources {
		s.byKey[src.Key] = NewsGroup{Key: src.Key, Label: src.Label}
	}
	s.itemHistory = make(map[string]NewsItem)
	s.itemOrder = nil
	s.fetchedAt = time.Time{}
}

func (s *techNewsStore) updateSource(src NewsSourceConfig, titles []string, now time.Time) {
	items := make([]NewsItem, 0, len(titles))
	for _, title := range titles {
		items = append(items, NewsItem{
			ID:          makeNewsItemID(src.Key, "", title),
			SourceKey:   src.Key,
			SourceLabel: src.Label,
			Title:       title,
		})
	}
	s.updateSourceItems(src, items, now)
}

func (s *techNewsStore) updateSourceItems(src NewsSourceConfig, items []NewsItem, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byKey == nil {
		s.byKey = make(map[string]NewsGroup)
	}
	if s.itemHistory == nil {
		s.itemHistory = make(map[string]NewsItem)
	}
	titles := make([]string, len(items))
	copied := make([]NewsItem, len(items))
	for i, item := range items {
		item.SourceKey = src.Key
		item.SourceLabel = src.Label
		if item.ID == "" {
			item.ID = makeNewsItemID(src.Key, item.Link, item.Title)
		}
		titles[i] = item.Title
		copied[i] = item
		if _, exists := s.itemHistory[item.ID]; !exists {
			s.itemOrder = append(s.itemOrder, item.ID)
		}
		s.itemHistory[item.ID] = item
	}
	for len(s.itemOrder) > newsMaxItemHistory {
		oldest := s.itemOrder[0]
		s.itemOrder = s.itemOrder[1:]
		delete(s.itemHistory, oldest)
	}
	s.byKey[src.Key] = NewsGroup{
		Key:       src.Key,
		Label:     src.Label,
		Titles:    titles,
		Items:     copied,
		FetchedAt: now,
	}
	s.fetchedAt = now
}

func (s *techNewsStore) failSource(src NewsSourceConfig, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byKey == nil {
		s.byKey = make(map[string]NewsGroup)
	}
	group, ok := s.byKey[src.Key]
	if !ok {
		group = NewsGroup{Key: src.Key, Label: src.Label}
	}
	group.Err = err.Error()
	s.byKey[src.Key] = group
}

// snapshot 返回按配置顺序排列的深拷贝快照。
func (s *techNewsStore) snapshot() TechNewsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := TechNewsSnapshot{
		FetchedAt: s.fetchedAt,
		Groups:    make([]NewsGroup, 0, len(s.sources)),
	}
	for _, src := range s.sources {
		group, ok := s.byKey[src.Key]
		if !ok {
			group = NewsGroup{Key: src.Key, Label: src.Label}
		} else {
			group.Titles = append([]string(nil), group.Titles...)
			group.Items = append([]NewsItem(nil), group.Items...)
		}
		if len(group.Titles) > 0 {
			out.Available = true
		}
		out.Groups = append(out.Groups, group)
	}
	return out
}

// findItemByID 按 ID 查找当前或最近缓存的资讯条目；未找到返回 false。
func (s *techNewsStore) findItemByID(id string) (NewsItem, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return NewsItem{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, src := range s.sources {
		if group, ok := s.byKey[src.Key]; ok {
			for _, item := range group.Items {
				if item.ID == id {
					return item, true
				}
			}
		}
	}
	if s.itemHistory != nil {
		item, ok := s.itemHistory[id]
		return item, ok
	}
	return NewsItem{}, false
}

// makeNewsItemID 基于来源键与链接（无链接时退回标题）生成 12 位小写十六进制稳定 ID。
func makeNewsItemID(sourceKey, link, title string) string {
	basis := strings.TrimSpace(link)
	if basis == "" {
		basis = strings.TrimSpace(title)
	}
	sum := sha256.Sum256([]byte(sourceKey + "|" + basis))
	return hex.EncodeToString(sum[:6])
}

// techNewsRevision 返回仅随页面可见内容变化的短哈希。
// 已有内容时忽略 FetchedAt 与单源瞬时错误，避免数据未变仍重绘右侧区域。
func techNewsRevision(snap TechNewsSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%t|", snap.Available)
	if !snap.Available {
		fmt.Fprintf(&b, "allErr=%t|", allNewsSourcesErrored(snap))
	}
	for _, group := range snap.Groups {
		fmt.Fprintf(&b, "%s:%s=", group.Key, group.Label)
		for _, title := range group.Titles {
			fmt.Fprintf(&b, "%s\n", title)
		}
		b.WriteByte(';')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:8])
}

func allNewsSourcesErrored(snap TechNewsSnapshot) bool {
	if len(snap.Groups) == 0 {
		return false
	}
	for _, group := range snap.Groups {
		if group.Err == "" {
			return false
		}
	}
	return true
}

// parseNewsSourcesConfig 解析 NEWS_SOURCES 配置字符串。
// 支持内置预设名（solidot,sspai,ithome,geekpark,v2ex）以及 "显示名=RSS地址" 格式，最多保留 5 个源。
func parseNewsSourcesConfig(raw string) []NewsSourceConfig {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = newsDefaultSourcesSpec
	}
	parts := strings.Split(trimmed, ",")
	out := make([]NewsSourceConfig, 0, newsMaxSources)
	seen := make(map[string]bool, newsMaxSources)
	customIdx := 1
	for _, entry := range parts {
		item := strings.TrimSpace(entry)
		if item == "" {
			continue
		}
		var src NewsSourceConfig
		if eq := strings.IndexByte(item, '='); eq >= 0 {
			label := strings.TrimSpace(item[:eq])
			rawURL := strings.TrimSpace(item[eq+1:])
			if label == "" || !isValidHTTPURL(rawURL) {
				log.Printf("NEWS_SOURCES 自定义源无效（%q），已忽略", item)
				continue
			}
			src = NewsSourceConfig{
				Key:   fmt.Sprintf("custom%d", customIdx),
				Label: label,
				URL:   rawURL,
				Kind:  "feed",
			}
			customIdx++
		} else {
			preset, ok := newsSourcePresets[strings.ToLower(item)]
			if !ok {
				log.Printf("NEWS_SOURCES 未知预设 %q，已忽略", item)
				continue
			}
			src = preset
		}
		if seen[src.Key] {
			continue
		}
		seen[src.Key] = true
		out = append(out, src)
		if len(out) >= newsMaxSources {
			break
		}
	}
	if len(out) == 0 && trimmed != newsDefaultSourcesSpec {
		return parseNewsSourcesConfig(newsDefaultSourcesSpec)
	}
	return out
}

// newsPollIntervalConfig 解析 NEWS_REFRESH_INTERVAL（秒）；小于 300 秒回退默认 30 分钟。
func newsPollIntervalConfig(raw string) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return newsDefaultInterval
	}
	seconds, err := strconv.Atoi(trimmed)
	if err != nil || seconds < int(newsMinInterval/time.Second) {
		log.Printf("NEWS_REFRESH_INTERVAL 无效（%q），使用默认 %d 秒", trimmed, int(newsDefaultInterval/time.Second))
		return newsDefaultInterval
	}
	return time.Duration(seconds) * time.Second
}

func isValidHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// startTechNewsRefresh 启动科技热榜后台定时抓取，首次抓取在后台执行以避免阻塞服务启动。
func startTechNewsRefresh(ctx context.Context) {
	sources := parseNewsSourcesConfig(os.Getenv("NEWS_SOURCES"))
	interval := newsPollIntervalConfig(os.Getenv("NEWS_REFRESH_INTERVAL"))
	techNews.setSources(sources)
	go func() {
		refreshTechNews(ctx, sources, &techNews)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refreshTechNews(ctx, sources, &techNews)
			}
		}
	}()
}

// refreshTechNews 并发抓取所有配置的科技源并分别更新缓存。
func refreshTechNews(ctx context.Context, sources []NewsSourceConfig, store *techNewsStore) {
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s NewsSourceConfig) {
			defer wg.Done()
			items, err := fetchNewsSourceItems(ctx, s)
			if err != nil {
				log.Printf("科技资讯抓取失败 [%s]: %v", s.Label, err)
				store.failSource(s, err)
				return
			}
			store.updateSourceItems(s, items, time.Now())
		}(src)
	}
	wg.Wait()
}

// fetchNewsSource 拉取并解析单个科技源的最新标题（最多 2 条）。
func fetchNewsSource(ctx context.Context, src NewsSourceConfig) ([]string, error) {
	items, err := fetchNewsSourceItems(ctx, src)
	if err != nil {
		return nil, err
	}
	return newsItemTitles(items), nil
}

// fetchNewsSourceItems 拉取并解析单个科技源的最新资讯条目（含标题、链接、日期与 RSS 正文）。
func fetchNewsSourceItems(ctx context.Context, src NewsSourceConfig) ([]NewsItem, error) {
	if !isValidHTTPURL(src.URL) {
		return nil, fmt.Errorf("资讯源 URL 非法")
	}
	reqCtx, cancel := context.WithTimeout(ctx, newsDefaultHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, src.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KindleDashboard/1.0)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, application/json;q=0.9, */*;q=0.8")
	client := &http.Client{Timeout: newsDefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, newsMaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if len(body) > newsMaxResponseBytes {
		return nil, fmt.Errorf("响应超过 %d 字节", newsMaxResponseBytes)
	}
	if src.Kind == "v2ex_json" {
		return parseV2EXHotJSONItems(body, src)
	}
	return parseNewsFeedItems(body, src)
}

func newsItemTitles(items []NewsItem) []string {
	titles := make([]string, len(items))
	for i, item := range items {
		titles[i] = item.Title
	}
	return titles
}

// parseNewsFeed 流式解析 RSS 2.0 / Atom XML，仅提取 item/entry 直属的标题。
func parseNewsFeed(body []byte, src NewsSourceConfig) ([]string, error) {
	items, err := parseNewsFeedItems(body, src)
	if err != nil {
		return nil, err
	}
	return newsItemTitles(items), nil
}

// parseNewsFeedItems 流式解析 RSS 2.0 / Atom XML，提取每个 item/entry 的标题、链接、日期和正文。
func parseNewsFeedItems(body []byte, src NewsSourceConfig) ([]NewsItem, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var stack []string
	var inItem bool
	var itemDepth int
	var captureField string
	var captureDepth int
	var captureBuf strings.Builder

	var curTitle string
	var curLink string
	var curLinkAlternate bool
	var curDate string
	var curEncoded string
	var curContent string
	var curDesc string
	var curSummary string

	resetItem := func() {
		curTitle = ""
		curLink = ""
		curLinkAlternate = false
		curDate = ""
		curEncoded = ""
		curContent = ""
		curDesc = ""
		curSummary = ""
	}

	items := make([]NewsItem, 0, newsItemsPerSource)
	seen := make(map[string]bool, newsItemsPerSource)

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(items) > 0 {
				break
			}
			return nil, fmt.Errorf("解析 XML 失败: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			if len(stack) == 0 && local == "html" {
				return nil, fmt.Errorf("返回了 HTML 页面而非 RSS/Atom")
			}
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, local)

			if !inItem && (local == "item" || local == "entry") {
				inItem = true
				itemDepth = len(stack)
				captureField = ""
				resetItem()
				continue
			}
			if !inItem {
				continue
			}
			if captureField != "" {
				continue
			}
			if parent == "item" || parent == "entry" {
				switch local {
				case "title", "link", "pubdate", "date", "published", "updated", "encoded", "content", "description", "summary":
					if local == "link" {
						href, rel := "", ""
						for _, attr := range t.Attr {
							switch strings.ToLower(attr.Name.Local) {
							case "href":
								href = strings.TrimSpace(attr.Value)
							case "rel":
								rel = strings.ToLower(strings.TrimSpace(attr.Value))
							}
						}
						if href != "" && (rel == "" || rel == "alternate") && (!curLinkAlternate || rel == "alternate") {
							curLink = href
							if rel == "alternate" {
								curLinkAlternate = true
							}
						}
					}
					captureField = local
					captureDepth = len(stack)
					captureBuf.Reset()
				}
			}
		case xml.CharData:
			if captureField != "" {
				captureBuf.Write(t)
			}
		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if captureField != "" && len(stack) < captureDepth {
				val := captureBuf.String()
				captureBuf.Reset()
				switch captureField {
				case "title":
					if curTitle == "" {
						curTitle = val
					}
				case "link":
					if curLink == "" && strings.TrimSpace(val) != "" {
						curLink = strings.TrimSpace(val)
					}
				case "pubdate", "date", "published", "updated":
					if curDate == "" {
						curDate = parseFeedDate(val)
					}
				case "encoded":
					curEncoded = val
				case "content":
					curContent = val
				case "description":
					curDesc = val
				case "summary":
					curSummary = val
				}
				captureField = ""
			} else if captureField != "" && isBodyField(captureField) && isBreakEndTag(local) {
				// 兼容未转义直接嵌入 XHTML/HTML 段落标签的 Atom/RSS 内容块。
				captureBuf.WriteByte('\n')
			}
			if inItem && len(stack) < itemDepth {
				inItem = false
				item := buildNewsItem(src, curTitle, curLink, curDate, curEncoded, curContent, curDesc, curSummary)
				resetItem()
				if item.Title == "" {
					continue
				}
				if src.FilterAds && isAdNewsTitle(item.Title) {
					continue
				}
				key := strings.ToLower(item.Title)
				if seen[key] {
					continue
				}
				seen[key] = true
				items = append(items, item)
				if len(items) >= newsItemsPerSource {
					return items, nil
				}
			}
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("未解析到有效标题")
	}
	return items, nil
}

func isBodyField(field string) bool {
	return field == "encoded" || field == "content" || field == "description" || field == "summary"
}

func isBreakEndTag(local string) bool {
	switch local {
	case "br", "p", "div", "li", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "tr":
		return true
	default:
		return false
	}
}

func buildNewsItem(src NewsSourceConfig, rawTitle, rawLink, date string, bodies ...string) NewsItem {
	title := cleanNewsTitle(rawTitle)
	if title == "" {
		return NewsItem{}
	}
	link := strings.TrimSpace(rawLink)
	if !isValidHTTPURL(link) {
		link = ""
	}
	bestBody := ""
	bestTrunc := false
	bestRunes := 0
	for _, rawBody := range bodies {
		if strings.TrimSpace(rawBody) == "" {
			continue
		}
		plain, trunc := extractPlainParagraphs(rawBody)
		n := len([]rune(plain))
		if n > bestRunes || (bestBody == "" && trunc) {
			bestBody = plain
			bestTrunc = trunc
			bestRunes = n
		}
	}
	return NewsItem{
		ID:            makeNewsItemID(src.Key, link, title),
		SourceKey:     src.Key,
		SourceLabel:   src.Label,
		Title:         title,
		Link:          link,
		PublishedAt:   date,
		FeedBody:      bestBody,
		FeedTruncated: bestTrunc,
	}
}

// parseV2EXHotJSON 解析 V2EX 热门主题 JSON 接口。
func parseV2EXHotJSON(body []byte, src NewsSourceConfig) ([]string, error) {
	items, err := parseV2EXHotJSONItems(body, src)
	if err != nil {
		return nil, err
	}
	return newsItemTitles(items), nil
}

// parseV2EXHotJSONItems 解析 V2EX 热门主题 JSON 接口并返回完整条目信息。
func parseV2EXHotJSONItems(body []byte, src NewsSourceConfig) ([]NewsItem, error) {
	var topics []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Created int64  `json:"created"`
	}
	if err := json.Unmarshal(body, &topics); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	items := make([]NewsItem, 0, newsItemsPerSource)
	seen := make(map[string]bool, newsItemsPerSource)
	for _, topic := range topics {
		date := ""
		if topic.Created > 0 {
			loc := appLoc
			if loc == nil {
				loc = time.FixedZone("UTC+8", 8*3600)
			}
			date = time.Unix(topic.Created, 0).In(loc).Format("2006-01-02")
		}
		item := buildNewsItem(src, topic.Title, topic.URL, date, topic.Content)
		if item.Title == "" {
			continue
		}
		if src.FilterAds && isAdNewsTitle(item.Title) {
			continue
		}
		key := strings.ToLower(item.Title)
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
		if len(items) >= newsItemsPerSource {
			break
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("未解析到有效标题")
	}
	return items, nil
}

// cleanNewsTitle 解码 HTML 实体，清除 Emoji / 变体选择符 / 控制字符并规整空白。
func cleanNewsTitle(raw string) string {
	s := html.UnescapeString(raw)
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) || isNewsNoiseRune(r) {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func isNewsNoiseRune(r rune) bool {
	if r == 0xFE0E || r == 0xFE0F || r == 0x200D || r == 0xFEFF {
		return true
	}
	if r >= 0x200B && r <= 0x200F {
		return true
	}
	if r >= 0x1F000 {
		return true
	}
	if (r >= 0x2600 && r <= 0x27BF) || (r >= 0x2300 && r <= 0x23FF) || (r >= 0x2B00 && r <= 0x2BFF) {
		return true
	}
	return false
}

// isAdNewsTitle 过滤电商导购、领券促销类标题，保留正常科技资讯。
func isAdNewsTitle(title string) bool {
	for _, kw := range newsAdKeywords {
		if strings.Contains(title, kw) {
			return true
		}
	}
	return false
}

var (
	reNewsScriptTag  = regexp.MustCompile(`(?is)<(script|style|noscript)[^>]*>.*?</\1>`)
	reNewsHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reNewsBlockTag   = regexp.MustCompile(`(?i)<br\s*/?>|</(?:p|div|li|h[1-6]|blockquote|tr|section|article|pre)>`)
	reNewsAnyTag     = regexp.MustCompile(`(?s)<[^>]+>`)
)

// extractPlainParagraphs 将 HTML 片段清洗为保留段落换行的纯文本，并识别常见摘要截断提示。
func extractPlainParagraphs(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", false
	}
	s := reNewsHTMLComment.ReplaceAllString(raw, " ")
	s = reNewsScriptTag.ReplaceAllString(s, " ")
	s = reNewsBlockTag.ReplaceAllString(s, "\n")
	s = reNewsAnyTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)

	var paragraphs []string
	truncated := false
	for _, rawLine := range strings.Split(s, "\n") {
		line := cleanNewsParagraphLine(rawLine)
		if line == "" {
			continue
		}
		for _, marker := range []string{"点击查看全文", "查看全文", "阅读原文", "查看原文"} {
			if line == marker || line == "["+marker+"]" {
				truncated = true
				line = ""
				break
			}
			if strings.HasSuffix(line, marker) {
				truncated = true
				line = strings.TrimSpace(strings.TrimSuffix(line, marker))
				break
			}
		}
		if line != "" {
			paragraphs = append(paragraphs, line)
		}
	}
	return strings.Join(paragraphs, "\n\n"), truncated
}

func cleanNewsParagraphLine(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	lastSpace := true
	for _, r := range raw {
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) || isNewsNoiseRune(r) {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

var feedDateLayouts = []string{
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC822,
	time.RFC3339Nano,
	time.RFC3339,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parseFeedDate(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	loc := appLoc
	if loc == nil {
		loc = time.FixedZone("UTC+8", 8*3600)
	}
	for _, layout := range feedDateLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.In(loc).Format("2006-01-02")
		}
	}
	return ""
}
