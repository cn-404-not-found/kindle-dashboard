// news_page.go —— 横屏「时钟天气 + 中文科技热榜」首页渲染
//
// 左栏与完整月历页完全一致（复用 drawLandscapeLeftColumn），右栏无顶栏、无底栏，
// 全高排布 5 个科技源 × 2 条 33px 粗体标题，末行基线与左下角节气平齐（y=1012）。
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
	"unicode"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
)

const (
	newsCanvasW = 1448.0
	newsCanvasH = 1072.0

	newsColLeft       = 691.0
	newsColRight      = 1393.0
	newsColWidth      = newsColRight - newsColLeft
	newsTopY          = 52.0
	newsBlockStep     = 200.0
	newsTitleSize     = 33.0
	newsBadgeSize     = 19.0
	newsBadgeH        = 32.0
	newsBadgeGap      = 12.0
	newsBulletX       = 695.0
	newsBulletSize    = 10.0
	newsSecondTextX   = 716.0
	newsSecondTextMax = newsColRight - newsSecondTextX
)

// renderLandscapeNewsDashboard 绘制「时钟天气 + 中文科技热榜」首页（1448×1072 PNG）。
func renderLandscapeNewsDashboard(data DashboardData, news TechNewsSnapshot) (result []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("渲染科技热榜页面失败: %v", p)
		}
	}()
	regular, err := loadCalendarFont(fontFile)
	if err != nil {
		return nil, err
	}
	bold, err := loadCalendarFont(boldFontFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		bold = regular
	}
	dc := gg.NewContext(int(newsCanvasW), int(newsCanvasH))
	dc.SetColor(color.White)
	dc.Clear()
	r := &renderer{dc: dc, reg: regular, bold: bold, faces: map[faceKey]font.Face{}}
	drawLandscapeLeftColumn(dc, r, data)

	black := color.Color(color.Black)
	white := color.Color(color.White)
	gray := color.RGBA{95, 95, 95, 255}
	bulletGray := color.RGBA{102, 102, 102, 255}
	dividerGray := color.RGBA{204, 204, 204, 255}

	blocks := selectNewsBlocks(news)
	if len(blocks) == 0 {
		msg := "科技资讯获取中..."
		if allNewsSourcesErrored(news) {
			msg = "科技资讯暂不可用"
		}
		r.text((newsColLeft+newsColRight)/2, newsCanvasH/2, msg, 32, gray, false, 0.5)
	} else {
		measure := func(s string) float64 {
			return r.measure(s, newsTitleSize, true)
		}
		for i, group := range blocks {
			blockY := newsTopY + float64(i)*newsBlockStep

			badgeW := math.Ceil(r.measure(group.Label, newsBadgeSize, true)) + 20
			if badgeW < 66 {
				badgeW = 66
			}
			if badgeW > 150 {
				badgeW = 150
			}
			dc.SetColor(black)
			dc.DrawRectangle(newsColLeft, blockY+4, badgeW, newsBadgeH)
			dc.Fill()
			r.fitText(newsColLeft+badgeW/2, blockY+27, group.Label, newsBadgeSize, badgeW-10, white, true, 0.5)

			firstX := newsColLeft + badgeW + newsBadgeGap
			firstMaxW := newsColRight - firstX
			l1, l2 := wrapTwoLines(measure, group.Titles[0], firstMaxW, newsColWidth)
			r.text(firstX, blockY+32, l1, newsTitleSize, black, true, 0)
			if l2 != "" {
				r.text(newsColLeft, blockY+73, l2, newsTitleSize, black, true, 0)
			}

			if len(group.Titles) > 1 {
				dc.SetColor(bulletGray)
				dc.DrawRectangle(newsBulletX, blockY+106, newsBulletSize, newsBulletSize)
				dc.Fill()
				s1, s2 := wrapTwoLines(measure, group.Titles[1], newsSecondTextMax, newsSecondTextMax)
				r.text(newsSecondTextX, blockY+119, s1, newsTitleSize, black, true, 0)
				if s2 != "" {
					r.text(newsSecondTextX, blockY+160, s2, newsTitleSize, black, true, 0)
				}
			}

			if i < len(blocks)-1 {
				dc.SetColor(dividerGray)
				dc.SetLineWidth(1)
				dc.DrawLine(newsColLeft, blockY+183, newsColRight, blockY+183)
				dc.Stroke()
			}
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, dc.Image()); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// selectNewsBlocks 挑选前 5 个已抓到有效标题的来源分组。
func selectNewsBlocks(news TechNewsSnapshot) []NewsGroup {
	blocks := make([]NewsGroup, 0, newsMaxSources)
	for _, group := range news.Groups {
		if len(group.Titles) == 0 {
			continue
		}
		blocks = append(blocks, group)
		if len(blocks) >= newsMaxSources {
			break
		}
	}
	return blocks
}

// wrapTwoLines 按像素宽度把标题折成最多两行；第二行仍超出时在末尾追加省略号。
func wrapTwoLines(measure func(string) float64, text string, firstMaxW, secondMaxW float64) (string, string) {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return "", ""
	}
	if measure(string(runes)) <= firstMaxW {
		return string(runes), ""
	}
	cut := fitRunePrefix(measure, runes, firstMaxW)
	// 若切在连续英文/数字单词中间，尽量回退到最近空格，避免单词断在半截。
	if cut < len(runes) && isWordRune(runes[cut-1]) && isWordRune(runes[cut]) {
		minCut := cut / 2
		if cut-8 > minCut {
			minCut = cut - 8
		}
		for i := cut - 1; i >= minCut; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
	}
	line1 := strings.TrimSpace(string(runes[:cut]))
	rest := []rune(strings.TrimSpace(string(runes[cut:])))
	if len(rest) == 0 {
		return line1, ""
	}
	if measure(string(rest)) <= secondMaxW {
		return line1, string(rest)
	}
	k := fitRunePrefix(measure, rest, secondMaxW)
	for k > 0 && measure(strings.TrimSpace(string(rest[:k]))+"…") > secondMaxW {
		k--
	}
	line2 := strings.TrimSpace(string(rest[:k]))
	return line1, line2 + "…"
}

func fitRunePrefix(measure func(string) float64, runes []rune, maxW float64) int {
	if len(runes) == 0 {
		return 0
	}
	lo, hi := 1, len(runes)
	best := 1
	for lo <= hi {
		mid := (lo + hi) / 2
		if measure(string(runes[:mid])) <= maxW {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || (r < unicode.MaxASCII && r == '-')
}
