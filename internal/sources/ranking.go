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
	MangaBZ      []MangaSearchResult `json:"mangabz"`
	DM5          []MangaSearchResult `json:"dm5"`
	CopyManga    []MangaSearchResult `json:"copymanga"`
	MangaBZErr   string              `json:"mangabz_err,omitempty"`
	DM5Err       string              `json:"dm5_err,omitempty"`
	CopyMangaErr string              `json:"copymanga_err,omitempty"`
}

var (
	homeCacheMu   sync.RWMutex
	homeCacheData HomeRankings
	homeCacheTime time.Time
)

// FetchMangaBZRank fetches top 12 real-time popular manga from MangaBZ
func (m *SourceManager) FetchMangaBZRank(ctx context.Context) ([]MangaSearchResult, error) {
	client := CreateHTTPClient(18 * time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.mangabz.com/manga-list-0-0-10-p1/", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/122.0.0.0")
	req.Header.Set("Referer", "https://www.mangabz.com/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 MangaBZ 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MangaBZ 返回状态码 %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	var results []MangaSearchResult
	seen := make(map[string]bool)

	doc.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
		if len(results) >= 12 {
			return
		}
		// Title link is inside h2.title > a, NOT the bare cover anchor (which wraps <img>)
		tLink := sel.Find("h2.title a, .mh-item-detali .title a").First()
		title := strings.TrimSpace(tLink.Text())
		if title == "" {
			// fallback: try the title attribute
			title, _ = tLink.Attr("title")
			title = strings.TrimSpace(title)
		}
		href, _ := tLink.Attr("href")
		if href == "" {
			// fallback: cover anchor href (e.g. /73bz/)
			href, _ = sel.Find("a").First().Attr("href")
		}
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
			author = "热门精选"
		}
		latest := strings.TrimSpace(sel.Find(".chapter a").Text())

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

	if len(results) == 0 {
		return nil, fmt.Errorf("未在 MangaBZ 页面提取到榜单列表")
	}

	return results, nil
}

// FetchDM5Rank fetches top 12 real-time popular manga from DM5 (动漫屋)
func (m *SourceManager) FetchDM5Rank(ctx context.Context) ([]MangaSearchResult, error) {
	client := CreateHTTPClient(20 * time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.dm5.com/manhua-rank/", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/122.0.0.0")
	req.Header.Set("Referer", "https://www.dm5.com/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 DM5 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("DM5 返回状态码 %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
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

	if len(results) < 12 {
		req2, err2 := http.NewRequestWithContext(ctx, "GET", "https://www.dm5.com/manhua-list-0-0-10/", nil)
		if err2 == nil {
			req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/122.0.0.0")
			req2.Header.Set("Referer", "https://www.dm5.com/")
			req2.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
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

	if len(results) == 0 {
		return nil, fmt.Errorf("未在 DM5 页面提取到榜单列表")
	}

	return results, nil
}

// FetchCopyMangaRank fetches top 12 popular/trending manga from CopyManga
func (m *SourceManager) FetchCopyMangaRank(ctx context.Context) ([]MangaSearchResult, error) {
	src, ok := m.GetSource("copymanga")
	if !ok {
		return nil, fmt.Errorf("拷贝漫画源未注册")
	}

	copySrc, ok := src.(*CopyMangaSource)
	if !ok {
		return nil, fmt.Errorf("无效的拷贝漫画源实例")
	}

	apiPath := "/api/v3/ranks?type=1&date_type=week&limit=12&offset=0&platform=3"
	resp, err := copySrc.doRequest(ctx, apiPath)
	if err != nil {
		return nil, fmt.Errorf("请求拷贝漫画失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("拷贝漫画返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取拷贝漫画响应失败: %w", err)
	}

	var data struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Results struct {
			List []struct {
				Comic struct {
					Name             string `json:"name"`
					PathWord         string `json:"path_word"`
					Cover            string `json:"cover"`
					Author           []struct{ Name string } `json:"author"`
					LastChapterTitle string                  `json:"last_chapter_title"`
				} `json:"comic"`
				Name     string `json:"name"`
				PathWord string `json:"path_word"`
				Cover    string `json:"cover"`
			} `json:"list"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("解析拷贝漫画 JSON 失败: %w", err)
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

	if len(results) == 0 {
		return nil, fmt.Errorf("拷贝漫画返回列表为空 (%s)", data.Message)
	}

	return results, nil
}

// GetHomeData compiles the homepage ranking across 3 major sources in real-time
func (m *SourceManager) GetHomeData(ctx context.Context) HomeRankings {
	homeCacheMu.RLock()
	if time.Since(homeCacheTime) < 5*time.Minute && (len(homeCacheData.MangaBZ) > 0 || len(homeCacheData.DM5) > 0) {
		data := homeCacheData
		homeCacheMu.RUnlock()
		return data
	}
	homeCacheMu.RUnlock()

	var wg sync.WaitGroup
	var mbz, dm5, copyM []MangaSearchResult
	var mbzErr, dm5Err, copyErr error

	// Each source gets its OWN 18s context — one slow/failing source won't cancel the others
	perSourceTimeout := 18 * time.Second

	wg.Add(3)
	go func() {
		defer wg.Done()
		sCtx, sCancel := context.WithTimeout(context.Background(), perSourceTimeout)
		defer sCancel()
		mbz, mbzErr = m.FetchMangaBZRank(sCtx)
	}()
	go func() {
		defer wg.Done()
		sCtx, sCancel := context.WithTimeout(context.Background(), perSourceTimeout)
		defer sCancel()
		dm5, dm5Err = m.FetchDM5Rank(sCtx)
	}()
	go func() {
		defer wg.Done()
		sCtx, sCancel := context.WithTimeout(context.Background(), perSourceTimeout)
		defer sCancel()
		copyM, copyErr = m.FetchCopyMangaRank(sCtx)
	}()

	// Wait for all goroutines, but cap the overall blocking at 20s
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		// Some sources timed out internally; proceed with whatever we have
	}

	if mbz == nil {
		mbz = make([]MangaSearchResult, 0)
	}
	if dm5 == nil {
		dm5 = make([]MangaSearchResult, 0)
	}
	if copyM == nil {
		copyM = make([]MangaSearchResult, 0)
	}

	result := HomeRankings{
		MangaBZ:   mbz,
		DM5:       dm5,
		CopyManga: copyM,
	}
	if mbzErr != nil {
		result.MangaBZErr = mbzErr.Error()
	}
	if dm5Err != nil {
		result.DM5Err = dm5Err.Error()
	}
	if copyErr != nil {
		result.CopyMangaErr = copyErr.Error()
	}

	homeCacheMu.Lock()
	homeCacheData = result
	homeCacheTime = time.Now()
	homeCacheMu.Unlock()

	return result
}
