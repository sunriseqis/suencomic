package sources

import "testing"

func makeCh(id, title string) ChapterInfo {
	return ChapterInfo{ID: id, Title: title}
}

func TestCleanAndSortChaptersDedupesTrial(t *testing.T) {
	in := []ChapterInfo{
		makeCh("c1", "第1话 试看"),
		makeCh("c2", "第1话"),
		makeCh("c3", "第2话"),
	}
	out := CleanAndSortChapters(in, "test")
	if len(out) != 2 {
		t.Fatalf("expected trial+full merged to 2 chapters, got %d: %+v", len(out), out)
	}
	if out[0].ID != "c2" {
		t.Fatalf("expected full version c2 kept, got %s", out[0].ID)
	}
}

func TestCleanAndSortChaptersGroupOrder(t *testing.T) {
	in := []ChapterInfo{
		makeCh("x1", "第1话"),
		makeCh("v1", "第1卷"),
		makeCh("e1", "番外 秋日篇"),
		makeCh("x2", "第2话"),
	}
	out := CleanAndSortChapters(in, "test")
	if len(out) != 4 {
		t.Fatalf("expected 4 chapters, got %d", len(out))
	}
	if out[0].Type != "volume" {
		t.Fatalf("expected volume group first, got %s", out[0].Type)
	}
	if out[len(out)-1].Type != "extra" {
		t.Fatalf("expected extra group last, got %s", out[len(out)-1].Type)
	}
	if out[1].ID != "x1" || out[2].ID != "x2" {
		t.Fatalf("expected chapters in numeric order, got %s, %s", out[1].ID, out[2].ID)
	}
}

func TestCleanAndSortChaptersDropsEmptyTitles(t *testing.T) {
	in := []ChapterInfo{
		makeCh("a", "开始阅读"),
		makeCh("b", ""),
		makeCh("c", "第3话"),
	}
	out := CleanAndSortChapters(in, "test")
	if len(out) != 1 || out[0].ID != "c" {
		t.Fatalf("expected only c to survive, got %+v", out)
	}
}

func TestSortSearchResultsByRelevanceExactFirst(t *testing.T) {
	results := []MangaSearchResult{
		{Title: "怪兽8号 特别篇", ID: "b"},
		{Title: "怪兽8号", ID: "a"},
		{Title: "关于我转生成为史莱姆这件事", ID: "c"},
	}
	sorted := SortSearchResultsByRelevance(results, "怪兽8号")
	if sorted[0].ID != "a" {
		t.Fatalf("expected exact match first, got %s", sorted[0].Title)
	}
	if sorted[1].ID != "b" {
		t.Fatalf("expected prefix match second, got %s", sorted[1].Title)
	}
}

func TestNormalizeChineseTraditionalToSimplified(t *testing.T) {
	// 鬼滅之刃 (traditional) should normalize identically to 鬼灭之刃
	if normalizeChinese("鬼滅之刃") != normalizeChinese("鬼灭之刃") {
		t.Fatalf("traditional/simplified normalization failed: %q vs %q",
			normalizeChinese("鬼滅之刃"), normalizeChinese("鬼灭之刃"))
	}
}
