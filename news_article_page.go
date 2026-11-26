// news_article_page.go —— 横屏全屏资讯正文阅读页排版、分页与渲染
//
// 画布与横屏看板同为 1448×1072，通过统一缩放链路适配 PW2（758×1024）与 PW3（1072×1448）：
//   - 顶栏保留「← 返回看板」按钮、来源标签、日期模式与当前页码；
//   - 首页展示完整标题（最多 3 行），续页改为单行深灰标题提示以释放更多正文空间；
//   - 正文 36px 粗体纯黑按 1288px 宽自动断行与多页分页，底栏提供翻页区域与 3 分钟返回提示；
//   - 全部使用黑白灰纯色渲染，不附带二维码。
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

const (
	articleCanvasW        = 1448.0
	articleCanvasH        = 1072.0
	articleBarLeft        = 64.0
	articleBarRight       = 1384.0
	articleContentLeft    = 80.0
	articleContentRight   = 1368.0
	articleContentW       = articleContentRight - articleContentLeft
	articleTopDividerY    = 106.0
	articleBottomDividerY = 978.0
	articleTitleSize      = 48.0
	articleTitleLineH     = 62.0
	articleFirstTitleY    = 186.0
	articleBodySize       = 36.0
	articleBodyLineH      = 60.0
	articleParaGap        = 24.0
	articleBodyMaxY       = 948.0
	articleNextBodyStartY = 232.0
)

type articleLine struct {
	Text        string
	IsParaStart bool
}

// renderNewsArticlePage 把单条资讯按 1448×1072 横屏阅读版式渲染为第 requestedPage 页的 PNG。
func renderNewsArticlePage(item NewsItem, body string, isFull bool, requestedPage int) (result []byte, pageNum int, totalPages int, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("渲染资讯阅读页失败: %v", p)
		}
	}()
	regular, err := loadCalendarFont(fontFile)
	if err != nil {
		return nil, 0, 0, err
	}
	bold, err := loadCalendarFont(boldFontFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, 0, 0, err
		}
		bold = regular
	}
	dc := gg.NewContext(int(articleCanvasW), int(articleCanvasH))
	dc.SetColor(color.White)
	dc.Clear()
	r := &renderer{dc: dc, reg: regular, bold: bold, faces: map[faceKey]font.Face{}}

	titleMeasure := func(s string) float64 {
		return r.measure(s, articleTitleSize, true)
	}
	bodyMeasure := func(s string) float64 {
		return r.measure(s, articleBodySize, true)
	}

	titleLines := wrapTitleLines(titleMeasure, item.Title, articleContentW, 3)
	if len(titleLines) == 0 {
		titleLines = []string{"科技资讯详情"}
	}
	lastTitleY := articleFirstTitleY + float64(len(titleLines)-1)*articleTitleLineH
	titleDividerY := lastTitleY + 34.0
	firstBodyStartY := titleDividerY + 62.0

	displayBody := strings.TrimSpace(body)
	if displayBody == "" {
		displayBody = "该条资讯暂未附带正文内容，请轻点左上角「返回看板」继续浏览。"
	}
	lines := buildArticleLines(bodyMeasure, displayBody, articleContentW)
	pages := paginateArticle(lines, articleBodyMaxY-firstBodyStartY, articleBodyMaxY-articleNextBodyStartY, articleBodyLineH, articleParaGap)

	totalPages = len(pages)
	pageNum = requestedPage
	if pageNum < 1 {
		pageNum = 1
	} else if pageNum > totalPages {
		pageNum = totalPages
	}

	black := color.Color(color.Black)
	white := color.Color(color.White)
	metaGray := color.RGBA{40, 40, 40, 255}
	dividerGray := color.RGBA{150, 150, 150, 255}
	bottomRuleGray := color.RGBA{140, 140, 140, 255}
	hintGray := color.RGBA{45, 45, 45, 255}
	disabledBorder := color.RGBA{204, 204, 204, 255}
	disabledText := color.RGBA{170, 170, 170, 255}

	// 顶栏：返回按钮 + 来源标签 + 日期状态 + 右侧页码
	dc.SetColor(black)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(articleBarLeft, 34, 198, 56, 8)
	dc.Stroke()
	r.text(articleBarLeft+99, 72, "← 返回看板", 28, black, true, 0.5)

	sourceLabel := strings.TrimSpace(item.SourceLabel)
	if sourceLabel == "" {
		sourceLabel = "科技热榜"
	}
	badgeW := math.Ceil(r.measure(sourceLabel, 26, true)) + 36
	if badgeW < 96 {
		badgeW = 96
	}
	if badgeW > 220 {
		badgeW = 220
	}
	const badgeX = 286.0
	dc.SetColor(black)
	dc.DrawRoundedRectangle(badgeX, 36, badgeW, 52, 6)
	dc.Fill()
	r.fitText(badgeX+badgeW/2, 71, sourceLabel, 26, badgeW-16, white, true, 0.5)

	modeLabel := "全文阅读"
	if !isFull && strings.TrimSpace(body) != "" {
		modeLabel = "摘要预览"
	}
	metaText := modeLabel
	if item.PublishedAt != "" {
		metaText = item.PublishedAt + " · " + modeLabel
	}
	r.fitText(badgeX+badgeW+22, 71, metaText, 26, 620-badgeW, metaGray, true, 0)
	r.text(articleBarRight, 72, fmt.Sprintf("第 %d / %d 页", pageNum, totalPages), 28, black, true, 1)

	dc.SetColor(black)
	dc.SetLineWidth(3)
	dc.DrawLine(articleBarLeft, articleTopDividerY, articleBarRight, articleTopDividerY)
	dc.Stroke()

	// 标题区：第 1 页全尺寸大字标题；第 2 页起折叠为单行提醒，把垂直空间留给正文。
	var y float64
	if pageNum == 1 {
		for idx, line := range titleLines {
			r.text(articleContentLeft, articleFirstTitleY+float64(idx)*articleTitleLineH, line, articleTitleSize, black, true, 0)
		}
		dc.SetColor(dividerGray)
		dc.SetLineWidth(1.5)
		dc.DrawLine(articleContentLeft, titleDividerY, articleContentRight, titleDividerY)
		dc.Stroke()
		y = firstBodyStartY
	} else {
		shortTitle := truncateLineWithEllipsis(func(s string) float64 {
			return r.measure(s, 26, true)
		}, item.Title, articleContentW)
		r.text(articleContentLeft, 152, shortTitle, 26, metaGray, true, 0)
		dc.SetColor(dividerGray)
		dc.SetLineWidth(1)
		dc.DrawLine(articleContentLeft, 172, articleContentRight, 172)
		dc.Stroke()
		y = articleNextBodyStartY
	}

	// 正文区
	curPageLines := pages[pageNum-1]
	for idx, line := range curPageLines {
		if idx > 0 {
			y += articleBodyLineH
			if line.IsParaStart {
				y += articleParaGap
			}
		}
		r.text(articleContentLeft, y, line.Text, articleBodySize, black, true, 0)
	}

	// 底栏：上一页 / 操作提示 / 下一页
	dc.SetColor(bottomRuleGray)
	dc.SetLineWidth(1.5)
	dc.DrawLine(articleBarLeft, articleBottomDividerY, articleBarRight, articleBottomDividerY)
	dc.Stroke()

	prevStroke, prevText, prevLineW := black, black, 2.0
	if pageNum <= 1 {
		prevStroke, prevText, prevLineW = disabledBorder, disabledText, 1.5
	}
	dc.SetColor(prevStroke)
	dc.SetLineWidth(prevLineW)
	dc.DrawRoundedRectangle(articleBarLeft, 994, 172, 52, 6)
	dc.Stroke()
	r.text(articleBarLeft+86, 1029, "‹ 上一页", 28, prevText, true, 0.5)

	r.fitText((articleBarLeft+articleBarRight)/2, 1029, "轻点底栏或左右滑翻页 · 3 分钟无操作自动返回看板", 24, 920, hintGray, true, 0.5)

	nextStroke, nextText, nextLineW := black, black, 2.0
	if pageNum >= totalPages {
		nextStroke, nextText, nextLineW = disabledBorder, disabledText, 1.5
	}
	dc.SetColor(nextStroke)
	dc.SetLineWidth(nextLineW)
	dc.DrawRoundedRectangle(articleBarRight-172, 994, 172, 52, 6)
	dc.Stroke()
	r.text(articleBarRight-86, 1029, "下一页 ›", 28, nextText, true, 0.5)

	var out bytes.Buffer
	if err := png.Encode(&out, dc.Image()); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), pageNum, totalPages, nil
}

func buildArticleLines(measure func(string) float64, body string, maxW float64) []articleLine {
	var all []articleLine
	for _, rawPara := range strings.Split(body, "\n") {
		para := strings.TrimSpace(rawPara)
		if para == "" {
			continue
		}
		wrapped := wrapParagraph(measure, para, maxW)
		for i, txt := range wrapped {
			all = append(all, articleLine{
				Text:        txt,
				IsParaStart: i == 0,
			})
		}
	}
	return all
}

func paginateArticle(lines []articleLine, firstMaxSpan, nextMaxSpan, lineStep, paraGap float64) [][]articleLine {
	if len(lines) == 0 {
		return [][]articleLine{{{Text: "", IsParaStart: true}}}
	}
	var pages [][]articleLine
	var cur []articleLine
	curSpan := 0.0
	limit := firstMaxSpan
	for _, line := range lines {
		if len(cur) == 0 {
			cur = append(cur, line)
			curSpan = 0
			continue
		}
		step := lineStep
		if line.IsParaStart {
			step += paraGap
		}
		if curSpan+step > limit {
			pages = append(pages, cur)
			cur = []articleLine{line}
			curSpan = 0
			limit = nextMaxSpan
			continue
		}
		curSpan += step
		cur = append(cur, line)
	}
	if len(cur) > 0 {
		pages = append(pages, cur)
	}
	return pages
}

func wrapTitleLines(measure func(string) float64, title string, maxW float64, maxLines int) []string {
	lines := wrapParagraph(measure, title, maxW)
	if len(lines) <= maxLines || maxLines <= 0 {
		return lines
	}
	kept := append([]string(nil), lines[:maxLines]...)
	kept[maxLines-1] = truncateLineWithEllipsis(measure, kept[maxLines-1]+"…", maxW)
	return kept
}

func wrapParagraph(measure func(string) float64, text string, maxW float64) []string {
	rem := []rune(strings.TrimSpace(text))
	if len(rem) == 0 {
		return nil
	}
	var lines []string
	for len(rem) > 0 {
		if measure(string(rem)) <= maxW {
			lines = append(lines, string(rem))
			break
		}
		cut := fitRunePrefix(measure, rem, maxW)
		// 仅沿连续 ASCII 词元回退，避免中文后紧跟英文词时过度早折。
		if cut > 0 && cut < len(rem) && isWordRune(rem[cut-1]) && isWordRune(rem[cut]) {
			start := cut - 1
			for start > 0 && isWordRune(rem[start-1]) {
				start--
			}
			if start > 0 && cut-start <= 12 && start >= cut/2 {
				cut = start
			}
		}
		// 避头点：避免下一行以句末或分隔标点开头。
		for cut > 2 && cut < len(rem) && isClosingPunctRune(rem[cut]) {
			cut--
		}
		line := strings.TrimSpace(string(rem[:cut]))
		if line != "" {
			lines = append(lines, line)
		}
		rem = []rune(strings.TrimSpace(string(rem[cut:])))
	}
	return lines
}

func truncateLineWithEllipsis(measure func(string) float64, text string, maxW float64) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || measure(trimmed) <= maxW {
		return trimmed
	}
	runes := []rune(trimmed)
	k := fitRunePrefix(measure, runes, maxW)
	for k > 0 && measure(strings.TrimSpace(string(runes[:k]))+"…") > maxW {
		k--
	}
	if k <= 0 {
		return "…"
	}
	return strings.TrimSpace(string(runes[:k])) + "…"
}
