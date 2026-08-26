package sources

import (
	"sort"
	"strings"
	"unicode"
)

// normalizeChinese replaces common Traditional Chinese characters with Simplified Chinese for uniform matching
func normalizeChinese(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"獸", "兽", "號", "号", "賊", "贼", "滅", "灭", "拳", "拳",
		"術", "术", "迴", "回", "戰", "战", "進", "进", "擊", "击",
		"體", "体", "傳", "传", "說", "说", "畫", "画", "話", "话",
		"卷", "卷", "冊", "册", "雙", "双", "龍", "龙", "門", "门",
		"開", "开", "關", "关", "東", "东", "車", "车", "長", "长",
		"點", "点", "電", "电", "鋸", "锯", "間", "间", "諜", "谍",
		"過", "过", "家", "家", "殺", "杀", "魔", "魔", "獵", "猎",
		"國", "国", "學", "学", "園", "园", "夢", "梦", "戀", "恋",
		"愛", "爱", "貓", "猫", "狗", "狗", "鳥", "鸟", "魚", "鱼",
	)
	s = replacer.Replace(s)

	// Strip whitespace and punctuation
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) && !unicode.IsSymbol(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isSubsequence checks if all runes in query appear in title in order
func isSubsequence(query, title string) bool {
	qRunes := []rune(query)
	tRunes := []rune(title)
	if len(qRunes) == 0 {
		return true
	}
	qi := 0
	for _, tr := range tRunes {
		if tr == qRunes[qi] {
			qi++
			if qi == len(qRunes) {
				return true
			}
		}
	}
	return false
}

// calculateRelevanceScore returns a score measuring how closely a title matches the search query
func calculateRelevanceScore(title, query string) int {
	normTitle := normalizeChinese(title)
	normQuery := normalizeChinese(query)

	if normTitle == "" || normQuery == "" {
		return 0
	}

	// 1. Exact match (Highest Priority)
	if normTitle == normQuery {
		return 10000
	}

	// 2. Starts with query (e.g. "怪兽8号外传" matching "怪兽8号")
	if strings.HasPrefix(normTitle, normQuery) {
		return 8000 - len(normTitle)
	}

	// 3. Query starts with title
	if strings.HasPrefix(normQuery, normTitle) {
		return 7000 - len(normTitle)
	}

	// 4. Contains query as substring (e.g. "奥特怪兽8号" matching "怪兽8号")
	if strings.Contains(normTitle, normQuery) {
		return 5000 - len(normTitle)
	}

	// 5. Query contains title as substring
	if strings.Contains(normQuery, normTitle) {
		return 4000 - len(normTitle)
	}

	// 6. Subsequence match (e.g. "怪8" matching "怪兽8号")
	if isSubsequence(normQuery, normTitle) {
		return 3000 - len(normTitle)
	}

	// 7. Partial match (some common characters)
	commonCount := 0
	for _, qr := range normQuery {
		if strings.ContainsRune(normTitle, qr) {
			commonCount++
		}
	}
	if commonCount > 0 {
		return 500 + commonCount*100 - len(normTitle)
	}

	return 10
}

// SortSearchResultsByRelevance sorts search results by keyword match relevance score descending
func SortSearchResultsByRelevance(results []MangaSearchResult, query string) []MangaSearchResult {
	if len(results) <= 1 {
		return results
	}

	type scoredItem struct {
		item  MangaSearchResult
		score int
	}

	scored := make([]scoredItem, len(results))
	for i, r := range results {
		scored[i] = scoredItem{
			item:  r,
			score: calculateRelevanceScore(r.Title, query),
		}
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		// Tie breaker: lower latency first
		if scored[i].item.LatencyMs != scored[j].item.LatencyMs && scored[i].item.LatencyMs > 0 && scored[j].item.LatencyMs > 0 {
			return scored[i].item.LatencyMs < scored[j].item.LatencyMs
		}
		return i < j
	})

	sorted := make([]MangaSearchResult, len(results))
	for i, s := range scored {
		sorted[i] = s.item
	}
	return sorted
}
