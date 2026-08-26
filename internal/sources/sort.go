package sources

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	chNumRegex = regexp.MustCompile(`(?i)(?:第|Ch\.?|Chapter\s*|Vol\.?|Volume\s*|#)?(\d+(?:\.\d+)?)(?:話|话|回|卷|冊|册|章|集|期)?`)
	volRegex   = regexp.MustCompile(`(?i)(?:第\s*\d+\s*[卷冊册]|Vol(?:\.|\s*)\d+|Volume\s*\d+|\b\d+\s*[卷冊册])`)
	extraRegex = regexp.MustCompile(`(?i)(?:番外|外傳|外传|特別|特别|短篇|百景|投票|設定|设定|插畫|插画|告知|後記|后记|SP|Special|附录|附錄|访谈|訪談|宣傳|宣传|宣傳片|特報|特报)`)
)

// ExtractChapterNumber extracts the numeric sequence from a chapter title
func ExtractChapterNumber(title string) float64 {
	m := chNumRegex.FindStringSubmatch(title)
	if len(m) > 1 {
		val, err := strconv.ParseFloat(m[1], 64)
		if err == nil {
			return val
		}
	}
	return -1
}

// ClassifyChapter determines whether a chapter is a volume (单行本), chapter (连载单话), or extra (番外特别篇)
func ClassifyChapter(title string) (chType string, groupName string) {
	t := strings.TrimSpace(title)

	// 1. Extra / Special
	if extraRegex.MatchString(t) {
		return "extra", "番外特别篇"
	}

	// 2. Volume (单行本/卷/册)
	if volRegex.MatchString(t) {
		return "volume", "单行本"
	}

	// 3. Serialized Chapter (连载单话)
	return "chapter", "连载单话"
}

// IsChapterTrial detects if a chapter is a preview/trial version
func IsChapterTrial(title string) bool {
	t := strings.ToLower(title)
	return strings.Contains(t, "试看") ||
		strings.Contains(t, "試看") ||
		strings.Contains(t, "预告") ||
		strings.Contains(t, "預告") ||
		strings.Contains(t, "预览") ||
		strings.Contains(t, "預覽") ||
		strings.Contains(t, "sample") ||
		strings.Contains(t, "preview")
}

// CleanAndSortChapters categorizes, deduplicates within groups, and sorts chapters
func CleanAndSortChapters(rawChapters []ChapterInfo, sourceID string) []ChapterInfo {
	if len(rawChapters) == 0 {
		return rawChapters
	}

	type itemHolder struct {
		num    float64
		chType string
		info   ChapterInfo
	}

	var validItems []itemHolder
	seenIDs := make(map[string]bool)

	for _, ch := range rawChapters {
		if ch.ID == "" || seenIDs[ch.ID] {
			continue
		}
		seenIDs[ch.ID] = true

		title := strings.TrimSpace(ch.Title)
		if title == "" || title == "開始閱讀" || title == "开始阅读" {
			continue
		}

		ch.Source = sourceID
		ch.IsTrial = IsChapterTrial(title)
		chType, groupName := ClassifyChapter(title)
		ch.Type = chType
		ch.Group = groupName
		num := ExtractChapterNumber(title)

		validItems = append(validItems, itemHolder{
			num:    num,
			chType: chType,
			info:   ch,
		})
	}

	// Deduplicate by (chType + num), favoring full version over trial
	// Volumes (第1卷) and Chapters (第1话) have different chTypes so they will NEVER collide!
	groupedByKey := make(map[string][]ChapterInfo)
	var noNumItems []ChapterInfo

	for _, v := range validItems {
		if v.num > 0 {
			key := fmt.Sprintf("%s_%.2f", v.chType, v.num)
			groupedByKey[key] = append(groupedByKey[key], v.info)
		} else {
			noNumItems = append(noNumItems, v.info)
		}
	}

	merged := make([]ChapterInfo, 0, len(validItems))
	for _, list := range groupedByKey {
		if len(list) == 1 {
			merged = append(merged, list[0])
			continue
		}
		// If multiple versions of the same chapter number exist (e.g. 试看 vs 完整), pick full version first
		var best ChapterInfo
		foundFull := false
		for _, c := range list {
			if !c.IsTrial && !foundFull {
				best = c
				foundFull = true
			} else if !foundFull {
				best = c
			}
		}
		merged = append(merged, best)
	}

	merged = append(merged, noNumItems...)

	// Sort logic:
	// Group precedence: volume (单行本) first, then chapter (连载单话), then extra (番外特别篇)
	// Within each group, sort by numeric chapter/volume index ascending (1 -> Latest)
	groupWeight := func(t string) int {
		switch t {
		case "volume":
			return 1
		case "chapter":
			return 2
		case "extra":
			return 3
		default:
			return 4
		}
	}

	sort.SliceStable(merged, func(i, j int) bool {
		gI := groupWeight(merged[i].Type)
		gJ := groupWeight(merged[j].Type)
		if gI != gJ {
			return gI < gJ
		}

		numI := ExtractChapterNumber(merged[i].Title)
		numJ := ExtractChapterNumber(merged[j].Title)
		if numI > 0 && numJ > 0 {
			if numI != numJ {
				return numI < numJ
			}
		}
		return merged[i].Order < merged[j].Order
	})

	// Re-assign 1-based order
	for i := range merged {
		merged[i].Order = i + 1
	}

	return merged
}
