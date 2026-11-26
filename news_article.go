// news_article.go —— 资讯正文按需抓取缓存、横屏触摸坐标转换与点击命中判定
//
// 职责：
//  1. 判断 RSS/Atom 条目是否已有完整正文；若仅含导语则按需抓取原文网页正文区并缓存；
//  2. 把 Kindle 设备触摸屏原始坐标（PW2 758×1024 / PW3 1072×1448 等竖屏坐标）
//     精确逆映射回 1448×1072 横屏逻辑坐标；
//  3. 提供 GET /api/tap 供 Kindle 端把轻点坐标翻译为翻页、打开正文或返回首页指令。
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	articleCacheTTL        = 2 * time.Hour
	articleCacheMaxEntries = 40
	articleFetchTimeout    = 10 * time.Second
	articleMinFeedRunes    = 180
)

type cachedArticle struct {
	body      string
	isFull    bool
	fetchedAt time.Time
}

type articleCacheStore struct {
	mu      sync.RWMutex
	entries map[string]cachedArticle
	order   []string
}

var articleCache articleCacheStore

func (c *articleCacheStore) get(id string, now time.Time) (cachedArticle, bool) {
	if id == "" {
		return cachedArticle{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.entries == nil {
		return cachedArticle{}, false
	}
	entry, ok := c.entries[id]
	if !ok || entry.fetchedAt.IsZero() || now.Sub(entry.fetchedAt) >= articleCacheTTL {
		return cachedArticle{}, false
	}
	return entry, true
}

func (c *articleCacheStore) set(id string, entry cachedArticle) {
	if id == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]cachedArticle)
	}
	if _, exists := c.entries[id]; !exists {
		c.order = append(c.order, id)
	}
	c.entries[id] = entry
	for len(c.order) > articleCacheMaxEntries {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}

func needsWebArticleFetch(item NewsItem) bool {
	if item.FeedTruncated {
		return true
	}
	return len([]rune(strings.TrimSpace(item.FeedBody))) < articleMinFeedRunes
}

// resolveArticleContent 返回可供全屏阅读的正文文本及是否为完整正文标记。
// RSS 已自带足够长正文时直接命中；否则尝试抓取原文网页正文区并写入内存缓存。
func resolveArticleContent(ctx context.Context, item NewsItem) (string, bool) {
	now := time.Now()
	if cached, ok := articleCache.get(item.ID, now); ok {
		return cached.body, cached.isFull
	}
	feedBody := strings.TrimSpace(item.FeedBody)
	if !needsWebArticleFetch(item) {
		articleCache.set(item.ID, cachedArticle{body: feedBody, isFull: true, fetchedAt: now})
		return feedBody, true
	}
	if isValidHTTPURL(item.Link) {
		webBody, err := fetchWebArticleBody(ctx, item.Link)
		if err == nil && webBody != "" {
			webRunes := len([]rune(webBody))
			feedRunes := len([]rune(feedBody))
			if feedRunes == 0 || webRunes > feedRunes+40 {
				articleCache.set(item.ID, cachedArticle{body: webBody, isFull: true, fetchedAt: now})
				return webBody, true
			}
		}
		if err != nil && feedBody != "" {
			// 原文网页临时抓取失败时不长期写入成功缓存，保留导语降级以便下次点击可重试。
			return feedBody, false
		}
	}
	if feedBody != "" {
		isFull := !item.FeedTruncated
		if !isValidHTTPURL(item.Link) || isFull {
			articleCache.set(item.ID, cachedArticle{body: feedBody, isFull: isFull, fetchedAt: now})
		}
		return feedBody, isFull
	}
	return "", false
}

func fetchWebArticleBody(ctx context.Context, rawURL string) (string, error) {
	if !isValidHTTPURL(rawURL) {
		return "", fmt.Errorf("文章 URL 非法")
	}
	reqCtx, cancel := context.WithTimeout(ctx, articleFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; KindleDashboard/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	client := &http.Client{Timeout: articleFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, newsMaxResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > newsMaxResponseBytes {
		return "", fmt.Errorf("网页响应过大")
	}
	text := extractWebArticleText(body)
	if text == "" {
		return "", fmt.Errorf("未提取到正文")
	}
	return text, nil
}

var (
	reArticleNonContentBlocks = compileTagBlockPatterns("script", "style", "noscript", "svg", "nav", "header", "footer", "aside")
	reArticleParagraphBlock   = regexp.MustCompile(`(?is)<p(?:\s[^>]*)?>(.*?)</p>`)
)

// extractWebArticleText 按常见中文科技站正文容器优先级提取纯文本段落。
func extractWebArticleText(rawHTML []byte) string {
	s := reNewsHTMLComment.ReplaceAllString(string(rawHTML), " ")
	s = stripRegexBlocks(s, reNewsScriptTags)

	attrContainers := []string{
		"wangEditor-txt",
		"post_content",
		`id="paragraph"`,
		`id='paragraph'`,
		"article-content",
		"entry-content",
		"topic_content",
		"markdown_body",
	}
	for _, attr := range attrContainers {
		if frag := extractElementByAttr(s, attr); frag != "" {
			cleanedFrag := stripRegexBlocks(frag, reArticleNonContentBlocks)
			if txt, _ := extractPlainParagraphs(cleanedFrag); len([]rune(txt)) >= 60 {
				return txt
			}
		}
	}
	for _, tag := range []string{"article", "main"} {
		if frag := extractTagBlock(s, tag); frag != "" {
			cleanedFrag := stripRegexBlocks(frag, reArticleNonContentBlocks)
			if txt, _ := extractPlainParagraphs(cleanedFrag); len([]rune(txt)) >= 60 {
				return txt
			}
		}
	}

	stripped := stripRegexBlocks(s, reArticleNonContentBlocks)
	matches := reArticleParagraphBlock.FindAllStringSubmatch(stripped, -1)
	var paras []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		p, _ := extractPlainParagraphs(m[1])
		if len([]rune(p)) >= 15 {
			paras = append(paras, p)
		}
	}
	return strings.Join(paras, "\n\n")
}

// asciiLower 只将 ASCII 大写字母转为小写，保证输出与原字符串字节偏移 1:1 对齐。
func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// extractElementByAttr 查找包含指定属性子串的 HTML 开始标签，并通过标签深度计数提取完整内容。
func extractElementByAttr(htmlStr, attrNeedle string) string {
	lower := asciiLower(htmlStr)
	target := asciiLower(attrNeedle)
	searchFrom := 0
	for searchFrom < len(lower) {
		rel := strings.Index(lower[searchFrom:], target)
		if rel < 0 {
			return ""
		}
		idx := searchFrom + rel
		openIdx := strings.LastIndexByte(htmlStr[:idx], '<')
		if openIdx < 0 || strings.IndexByte(htmlStr[openIdx:idx], '>') >= 0 {
			searchFrom = idx + len(target)
			continue
		}
		tagEndRel := strings.IndexByte(htmlStr[idx:], '>')
		if tagEndRel < 0 {
			return ""
		}
		tagEnd := idx + tagEndRel
		if tagEnd > openIdx && htmlStr[tagEnd-1] == '/' {
			searchFrom = tagEnd + 1
			continue
		}
		tagName := parseHTMLTagName(htmlStr[openIdx+1 : idx])
		if tagName == "" {
			searchFrom = tagEnd + 1
			continue
		}
		if inner := scanBalancedTagInner(htmlStr, lower, tagName, tagEnd+1); inner != "" {
			return inner
		}
		searchFrom = tagEnd + 1
	}
	return ""
}

func extractTagBlock(htmlStr, tagName string) string {
	lower := asciiLower(htmlStr)
	tagLower := asciiLower(tagName)
	openPrefix := "<" + tagLower
	searchFrom := 0
	for searchFrom < len(lower) {
		rel := strings.Index(lower[searchFrom:], openPrefix)
		if rel < 0 {
			return ""
		}
		openIdx := searchFrom + rel
		after := openIdx + len(openPrefix)
		if after >= len(lower) {
			return ""
		}
		nextCh := lower[after]
		if nextCh != '>' && !isHTMLSpace(nextCh) {
			searchFrom = after
			continue
		}
		tagEndRel := strings.IndexByte(htmlStr[after:], '>')
		if tagEndRel < 0 {
			return ""
		}
		tagEnd := after + tagEndRel
		if inner := scanBalancedTagInner(htmlStr, lower, tagLower, tagEnd+1); inner != "" {
			return inner
		}
		searchFrom = tagEnd + 1
	}
	return ""
}

func scanBalancedTagInner(htmlStr, lower, tagName string, contentStart int) string {
	tagLower := strings.ToLower(tagName)
	openPat := "<" + tagLower
	closePat := "</" + tagLower
	depth := 1
	pos := contentStart
	for pos < len(lower) {
		rel := strings.IndexByte(lower[pos:], '<')
		if rel < 0 {
			return ""
		}
		pos += rel
		if strings.HasPrefix(lower[pos:], closePat) {
			afterClose := pos + len(closePat)
			if afterClose < len(lower) && (lower[afterClose] == '>' || isHTMLSpace(lower[afterClose])) {
				depth--
				endRel := strings.IndexByte(lower[afterClose:], '>')
				if depth == 0 {
					return htmlStr[contentStart:pos]
				}
				if endRel < 0 {
					return ""
				}
				pos = afterClose + endRel + 1
				continue
			}
		} else if strings.HasPrefix(lower[pos:], openPat) {
			afterOpen := pos + len(openPat)
			if afterOpen < len(lower) && (lower[afterOpen] == '>' || lower[afterOpen] == '/' || isHTMLSpace(lower[afterOpen])) {
				endRel := strings.IndexByte(lower[afterOpen:], '>')
				if endRel < 0 {
					return ""
				}
				endPos := afterOpen + endRel
				if endPos > pos && lower[endPos-1] != '/' {
					depth++
				}
				pos = endPos + 1
				continue
			}
		}
		pos++
	}
	return ""
}

func parseHTMLTagName(fragment string) string {
	s := strings.TrimSpace(fragment)
	end := 0
	for end < len(s) {
		ch := s[end]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			end++
			continue
		}
		break
	}
	return strings.ToLower(s[:end])
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// rawTouchToLandscape 把设备竖屏原始触摸坐标换算回 1448×1072 横屏逻辑坐标。
// 对应 main.go 中 fitToDevice 的 rotate90CW 变换（dst(1071-ly, lx)）。
func rawTouchToLandscape(rawX, rawY, fbW, fbH int) (lx, ly float64, ok bool) {
	if fbW <= 0 || fbH <= 0 || rawX < 0 || rawY < 0 || rawX > fbW || rawY > fbH {
		return 0, 0, false
	}
	px := float64(rawX) * landscapePortraitW / float64(fbW)
	py := float64(rawY) * landscapePortraitH / float64(fbH)
	lx = py
	ly = (landscapePortraitW - 1) - px
	if lx < 0 {
		lx = 0
	} else if lx > landscapeCanvasW-1 {
		lx = landscapeCanvasW - 1
	}
	if ly < 0 {
		ly = 0
	} else if ly > landscapeCanvasH-1 {
		ly = landscapeCanvasH - 1
	}
	return lx, ly, true
}

const (
	landscapePortraitW = 1072.0
	landscapePortraitH = 1448.0
	landscapeCanvasW   = 1448.0
	landscapeCanvasH   = 1072.0
)

// hitTestHomeNews 判定横屏首页一次轻点是否落在右侧 5 组 × 2 条新闻的点击热区内。
func hitTestHomeNews(lx, ly float64, news TechNewsSnapshot) (NewsItem, bool) {
	if lx < 660 || lx > 1440 {
		return NewsItem{}, false
	}
	blocks := selectNewsBlocks(news)
	for i, group := range blocks {
		blockY := newsTopY + float64(i)*newsBlockStep
		top := blockY - 12
		if i == 0 {
			top = 20
		}
		bottom := blockY + 188
		if i == len(blocks)-1 {
			bottom = 1055
		}
		if ly < top || ly >= bottom {
			continue
		}
		itemIdx := 0
		if ly >= blockY+90 {
			itemIdx = 1
		}
		if itemIdx >= len(group.Items) {
			return NewsItem{}, false
		}
		item := group.Items[itemIdx]
		if item.ID == "" {
			return NewsItem{}, false
		}
		return item, true
	}
	return NewsItem{}, false
}

// hitTestArticleDetail 判定全屏正文阅读页上的轻点动作（返回看板、上一页、下一页或 noop）。
func hitTestArticleDetail(lx, ly float64, articleID string, curPage, totalPages int) string {
	if ly <= 125 && lx <= 480 {
		return "home"
	}
	if ly >= 940 {
		if lx < 520 {
			if articleID != "" && curPage > 1 {
				return fmt.Sprintf("open %s %d", articleID, curPage-1)
			}
			return "noop"
		}
		if lx >= 928 {
			if articleID != "" && totalPages > 0 && curPage < totalPages {
				return fmt.Sprintf("open %s %d", articleID, curPage+1)
			}
			return "noop"
		}
	}
	return "noop"
}

// tapHandler 处理 GET /api/tap：解析设备轻点坐标并输出单行控制指令（open/home/noop）。
func tapHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	rawX, errX := strconv.Atoi(q.Get("x"))
	rawY, errY := strconv.Atoi(q.Get("y"))
	fbW, errW := strconv.Atoi(q.Get("w"))
	fbH, errH := strconv.Atoi(q.Get("h"))

	action := "noop"
	if errX == nil && errY == nil && errW == nil && errH == nil {
		if lx, ly, ok := rawTouchToLandscape(rawX, rawY, fbW, fbH); ok {
			page := q.Get("page")
			switch page {
			case "", dashboardPageHome:
				if item, hit := hitTestHomeNews(lx, ly, techNews.snapshot()); hit {
					action = fmt.Sprintf("open %s 1", item.ID)
				}
			case dashboardPageNewsDetail:
				articleID := strings.TrimSpace(q.Get("id"))
				curPage, _ := strconv.Atoi(q.Get("p"))
				if curPage <= 0 {
					curPage = 1
				}
				totalPages, _ := strconv.Atoi(q.Get("total"))
				action = hitTestArticleDetail(lx, ly, articleID, curPage, totalPages)
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, action+"\n")
}
