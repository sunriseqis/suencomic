package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type HomeRankings struct {
	Shueisha  []MangaSearchResult `json:"shueisha"`
	MangaBZ   []MangaSearchResult `json:"mangabz"`
	DM5       []MangaSearchResult `json:"dm5"`
	CopyManga []MangaSearchResult `json:"copymanga"`
	Pica      []MangaSearchResult `json:"pica"`
}

// Fixed flagship Shueisha / Shōnen Jump masterpieces catalog for guaranteed instant zero-latency loading
var ShueishaCatalog = []MangaSearchResult{
	{
		ID:            "139bz",
		Title:         "海贼王 (ONE PIECE)",
		Cover:         "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg",
		Author:        "尾田荣一郎 (集英社)",
		LatestChapter: "周刊少年Jump 连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "73bz",
		Title:         "鬼灭之刃",
		Cover:         "https://cover.mangabz.com/1/73/20191206092901_180x240_25.jpg",
		Author:        "吾峠呼世晴 (集英社)",
		LatestChapter: "全206话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "38bz",
		Title:         "一拳超人",
		Cover:         "https://cover.mangabz.com/1/38/20191206093227_180x240_21.jpg",
		Author:        "ONE / 村田雄介 (集英社)",
		LatestChapter: "连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "266bz",
		Title:         "咒术回战",
		Cover:         "https://cover.mangabz.com/1/266/20191203170525_180x240_26.jpg",
		Author:        "芥见下下 (集英社)",
		LatestChapter: "周刊少年Jump 连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "577bz",
		Title:         "电锯人 (链锯人)",
		Cover:         "https://cover.mangabz.com/1/577/20191207091649_180x240_24.jpg",
		Author:        "藤本树 (集英社)",
		LatestChapter: "少年Jump+ 连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "611bz",
		Title:         "SPY×FAMILY 间谍过家家",
		Cover:         "https://cover.mangabz.com/1/611/20191207105549_180x240_16.jpg",
		Author:        "远藤达哉 (集英社)",
		LatestChapter: "少年Jump+ 连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "142bz",
		Title:         "火影忍者 (NARUTO)",
		Cover:         "https://cover.mangabz.com/1/142/20191202152947_180x240_28.jpg",
		Author:        "岸本齐史 (集英社)",
		LatestChapter: "全700话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "1bz",
		Title:         "死神 (BLEACH / 境·界)",
		Cover:         "https://cover.mangabz.com/1/1/20200101121446_180x240_22.jpg",
		Author:        "久保带人 (集英社)",
		LatestChapter: "全686话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "892bz",
		Title:         "灌篮高手 (SLAM DUNK)",
		Cover:         "https://cover.mangabz.com/1/892/20191119100653_180x240_23.jpg",
		Author:        "井上雄彦 (集英社)",
		LatestChapter: "全31卷 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "440bz",
		Title:         "龙珠 (DRAGON BALL)",
		Cover:         "https://cover.mangabz.com/1/440/20191204170434_180x240_25.jpg",
		Author:        "鸟山明 (集英社)",
		LatestChapter: "全519话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "46899bz",
		Title:         "全职猎人 (HUNTER×HUNTER)",
		Cover:         "https://cover.mangabz.com/47/46899/20260418090527_180x240_24.jpg",
		Author:        "富坚义博 (集英社)",
		LatestChapter: "周刊少年Jump",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "263bz",
		Title:         "排球少年！！",
		Cover:         "https://cover.mangabz.com/1/263/20191203165851_180x240_22.jpg",
		Author:        "古馆春一 (集英社)",
		LatestChapter: "全402话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
}

// 15-Minute in-memory cache for ultra-fast homepage loading and anti-rate-limiting
var (
	homeCacheMu   sync.RWMutex
	homeCacheData HomeRankings
	homeCacheTime time.Time
)

// FetchMangaBZRank fetches top 12 real-time popular manga from MangaBZ
func (m *SourceManager) FetchMangaBZRank(ctx context.Context) []MangaSearchResult {
	client := CreateHTTPClient(6 * time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.mangabz.com/manga-list-0-0-10-p1/", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://www.mangabz.com/")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	var results []MangaSearchResult
	seen := make(map[string]bool)

	doc.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
		if len(results) >= 12 {
			return
		}
		tLink := sel.Find(".title a, h2.title a").First()
		title := strings.TrimSpace(tLink.Text())
		href, _ := tLink.Attr("href")
		if title == "" || href == "" {
			return
		}
		id := strings.Trim(href, "/")
		if seen[id] || seen[title] {
			return
		}
		seen[id] = true
		seen[title] = true

		cover, _ := sel.Find("img.mh-cover, img").Attr("src")
		author := strings.TrimSpace(sel.Find(".author").Text())
		if author == "" {
			author = "人气精选"
		}
		latest := strings.TrimSpace(sel.Find(".chapter a, .title a:nth-child(2)").Text())

		results = append(results, MangaSearchResult{
			ID:            id,
			Title:         title,
			Cover:         cover,
			Author:        author,
			LatestChapter: latest,
			Source:        "mangabz",
			SourceName:    "MangaBZ",
		})
	})

	return results
}

// FetchDM5Rank fetches top 12 real-time popular manga from DM5 (动漫屋)
func (m *SourceManager) FetchDM5Rank(ctx context.Context) []MangaSearchResult {
	client := CreateHTTPClient(6 * time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.dm5.com/manhua-rank/", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Referer", "https://www.dm5.com/")

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil
	}

	bgURLRegex := regexp.MustCompile(`url\(([^)]+)\)`)
	var results []MangaSearchResult
	seen := make(map[string]bool)

	parseDM5Items := func(d *goquery.Document) {
		d.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
			if len(results) >= 12 {
				return
			}
			tLink := sel.Find("h2.title a, .title a").First()
			title := strings.TrimSpace(tLink.Text())
			href, _ := tLink.Attr("href")
			if title == "" || href == "" {
				return
			}
			id := strings.Trim(href, "/")
			if seen[id] || seen[title] {
				return
			}
			seen[id] = true
			seen[title] = true

			// Extract cover from img or background-image style
			cover, _ := sel.Find("img").Attr("src")
			if cover == "" {
				style, _ := sel.Find(".mh-cover").Attr("style")
				if match := bgURLRegex.FindStringSubmatch(style); len(match) > 1 {
					cover = strings.Trim(match[1], `"' `)
				}
			}
			author := strings.TrimSpace(sel.Find(".zl, .author").Text())
			author = strings.TrimPrefix(author, "作者：")
			if author == "" {
				author = "动漫屋热门"
			}
			latest := strings.TrimSpace(sel.Find(".chapter a").Text())

			results = append(results, MangaSearchResult{
				ID:            id,
				Title:         title,
				Cover:         cover,
				Author:        author,
				LatestChapter: latest,
				Source:        "dm5",
				SourceName:    "DM5",
			})
		})
	}

	parseDM5Items(doc)

	// If fewer than 12 unique items from rank page, fill with DM5 popular catalog
	if len(results) < 12 {
		req2, err2 := http.NewRequestWithContext(ctx, "GET", "https://www.dm5.com/manhua-list-0-0-10/", nil)
		if err2 == nil {
			req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			req2.Header.Set("Referer", "https://www.dm5.com/")
			resp2, rErr2 := client.Do(req2)
			if rErr2 == nil && resp2.StatusCode == 200 {
				doc2, dErr2 := goquery.NewDocumentFromReader(resp2.Body)
				resp2.Body.Close()
				if dErr2 == nil {
					parseDM5Items(doc2)
				}
			}
		}
	}

	return results
}

// FetchCopyMangaRank fetches top 12 popular/trending manga from CopyManga
func (m *SourceManager) FetchCopyMangaRank(ctx context.Context) []MangaSearchResult {
	src, ok := m.GetSource("copymanga")
	if !ok {
		return nil
	}

	copySrc, ok := src.(*CopyMangaSource)
	if !ok {
		return nil
	}

	apiPath := "/api/v3/ranks?type=1&date_type=week&limit=12&offset=0&platform=3"
	resp, err := copySrc.doRequest(ctx, apiPath)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		// Fallback to searching top hits
		searchRes, sErr := copySrc.Search(ctx, "超人气")
		if sErr == nil && len(searchRes) > 0 {
			if len(searchRes) > 12 {
				return searchRes[:12]
			}
			return searchRes
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var data struct {
		Code    int `json:"code"`
		Results struct {
			List []struct {
				Comic struct {
					Name     string `json:"name"`
					PathWord string `json:"path_word"`
					Cover    string `json:"cover"`
					Author   []struct {
						Name string `json:"name"`
					} `json:"author"`
					LastChapterTitle string `json:"last_chapter_title"`
				} `json:"comic"`
				Name     string `json:"name"`
				PathWord string `json:"path_word"`
				Cover    string `json:"cover"`
			} `json:"list"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	var results []MangaSearchResult
	for _, item := range data.Results.List {
		if len(results) >= 12 {
			break
		}
		c := item.Comic
		name := c.Name
		if name == "" {
			name = item.Name
		}
		path := c.PathWord
		if path == "" {
			path = item.PathWord
		}
		cover := c.Cover
		if cover == "" {
			cover = item.Cover
		}
		author := "拷贝热门"
		if len(c.Author) > 0 && c.Author[0].Name != "" {
			author = c.Author[0].Name
		}

		if name != "" && path != "" {
			results = append(results, MangaSearchResult{
				ID:            path,
				Title:         name,
				Cover:         cover,
				Author:        author,
				LatestChapter: c.LastChapterTitle,
				Source:        "copymanga",
				SourceName:    "CopyManga",
			})
		}
	}

	return results
}

// FetchPicaRank fetches top 12 popular/leaderboard manga from PicAcg (哔咔)
func (m *SourceManager) FetchPicaRank(ctx context.Context) []MangaSearchResult {
	src, ok := m.GetSource("pica")
	if !ok {
		return nil
	}

	picaSrc, ok := src.(*PicaSource)
	if !ok {
		return nil
	}

	// Ensure login token is available
	_ = picaSrc.ensureLogin(ctx)

	// Try fetching 24H leaderboard
	resp, err := picaSrc.doRequest(ctx, "GET", "comics/leaderboard?tt=H24&ct=VC", nil)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		// Fallback to random / popular comics
		res, sErr := picaSrc.Search(ctx, "精选")
		if sErr == nil && len(res) > 0 {
			if len(res) > 12 {
				return res[:12]
			}
			return res
		}
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var data struct {
		Code int `json:"code"`
		Data struct {
			Comics []struct {
				ID         string `json:"_id"`
				Title      string `json:"title"`
				Author     string `json:"author"`
				Thumb      struct {
					FileServer string `json:"fileServer"`
					Path       string `json:"path"`
				} `json:"thumb"`
				Categories []string `json:"categories"`
				PagesCount int      `json:"pagesCount"`
			} `json:"comics"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}

	var results []MangaSearchResult
	for _, c := range data.Data.Comics {
		if len(results) >= 12 {
			break
		}
		cover := ""
		if c.Thumb.FileServer != "" && c.Thumb.Path != "" {
			cover = fmt.Sprintf("%s/static/%s", strings.TrimRight(c.Thumb.FileServer, "/"), strings.TrimLeft(c.Thumb.Path, "/"))
		}
		author := c.Author
		if author == "" {
			author = "哔咔精选"
		}
		results = append(results, MangaSearchResult{
			ID:            c.ID,
			Title:         c.Title,
			Cover:         cover,
			Author:        author,
			LatestChapter: fmt.Sprintf("%dP 完本", c.PagesCount),
			Source:        "pica",
			SourceName:    "PicAcg",
		})
	}

	return results
}

// GetHomeData compiles the homepage ranking across 4 major sources with 15-min in-memory cache
func (m *SourceManager) GetHomeData(ctx context.Context) HomeRankings {
	homeCacheMu.RLock()
	if time.Since(homeCacheTime) < 15*time.Minute && len(homeCacheData.MangaBZ) > 0 {
		data := homeCacheData
		homeCacheMu.RUnlock()
		return data
	}
	homeCacheMu.RUnlock()

	fetchCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mbz, dm5, copyM, pica []MangaSearchResult

	// Fetch 4 sources concurrently
	wg.Add(4)
	go func() {
		defer wg.Done()
		mbz = m.FetchMangaBZRank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		dm5 = m.FetchDM5Rank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		copyM = m.FetchCopyMangaRank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		pica = m.FetchPicaRank(fetchCtx)
	}()

	wg.Wait()

	// If a source returned empty (due to offline or initial loading), provide graceful fallbacks
	if len(mbz) == 0 {
		mbz = ShueishaCatalog
	}
	if len(dm5) == 0 {
		dm5 = ShueishaCatalog
	}
	if len(copyM) == 0 {
		copyM = ShueishaCatalog
	}
	if len(pica) == 0 {
		pica = ShueishaCatalog
	}

	result := HomeRankings{
		Shueisha:  ShueishaCatalog,
		MangaBZ:   mbz,
		DM5:       dm5,
		CopyManga: copyM,
		Pica:      pica,
	}

	homeCacheMu.Lock()
	homeCacheData = result
	homeCacheTime = time.Now()
	homeCacheMu.Unlock()

	return result
}
