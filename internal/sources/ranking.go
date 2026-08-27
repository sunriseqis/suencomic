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
	MangaBZ   []MangaSearchResult `json:"mangabz"`
	DM5       []MangaSearchResult `json:"dm5"`
	CopyManga []MangaSearchResult `json:"copymanga"`
	Pica      []MangaSearchResult `json:"pica"`
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

	if mbz == nil {
		mbz = make([]MangaSearchResult, 0)
	}
	if dm5 == nil {
		dm5 = make([]MangaSearchResult, 0)
	}
	if copyM == nil {
		copyM = make([]MangaSearchResult, 0)
	}
	if pica == nil {
		pica = make([]MangaSearchResult, 0)
	}

	result := HomeRankings{
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
