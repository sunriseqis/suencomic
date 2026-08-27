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
	Pica         []MangaSearchResult `json:"pica"`
	MangaBZErr   string              `json:"mangabz_err,omitempty"`
	DM5Err       string              `json:"dm5_err,omitempty"`
	CopyMangaErr string              `json:"copymanga_err,omitempty"`
	PicaErr      string              `json:"pica_err,omitempty"`
}

var (
	homeCacheMu   sync.RWMutex
	homeCacheData HomeRankings
	homeCacheTime time.Time
)

// FetchMangaBZRank fetches top 12 real-time popular manga from MangaBZ
func (m *SourceManager) FetchMangaBZRank(ctx context.Context) ([]MangaSearchResult, error) {
	client := CreateHTTPClient(10 * time.Second)
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
		tLink := sel.Find(".title a, h2.title a, a").First()
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
			author = "热门精选"
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

	if len(results) == 0 {
		return nil, fmt.Errorf("未在 MangaBZ 页面提取到榜单列表")
	}

	return results, nil
}

// FetchDM5Rank fetches top 12 real-time popular manga from DM5 (动漫屋)
func (m *SourceManager) FetchDM5Rank(ctx context.Context) ([]MangaSearchResult, error) {
	client := CreateHTTPClient(10 * time.Second)
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

// FetchPicaRank fetches top 12 popular/leaderboard manga from PicAcg (哔咔)
func (m *SourceManager) FetchPicaRank(ctx context.Context) ([]MangaSearchResult, error) {
	src, ok := m.GetSource("pica")
	if !ok {
		return nil, fmt.Errorf("哔咔漫画源未注册")
	}

	picaSrc, ok := src.(*PicaSource)
	if !ok {
		return nil, fmt.Errorf("无效的哔咔漫画源实例")
	}

	if err := picaSrc.ensureLogin(ctx); err != nil {
		return nil, fmt.Errorf("哔咔认证失败: %w", err)
	}

	resp, err := picaSrc.doRequest(ctx, "GET", "comics/leaderboard?tt=H24&ct=VC", nil)
	if err != nil {
		return nil, fmt.Errorf("获取哔咔排行榜失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("哔咔返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取哔咔响应失败: %w", err)
	}

	var data struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Comics []struct {
				ID         string `json:"_id"`
				Title      string `json:"title"`
				Author     string `json:"author"`
				Thumb      struct {
					FileServer string `json:"fileServer"`
					Path       string `json:"path"`
				} `json:"thumb"`
				PagesCount int `json:"pagesCount"`
			} `json:"comics"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("解析哔咔 JSON 失败: %w", err)
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

	if len(results) == 0 {
		return nil, fmt.Errorf("哔咔返回列表为空 (%s)", data.Message)
	}

	return results, nil
}

// GetHomeData compiles the homepage ranking across 4 major sources in real-time
func (m *SourceManager) GetHomeData(ctx context.Context) HomeRankings {
	homeCacheMu.RLock()
	if time.Since(homeCacheTime) < 5*time.Minute && (len(homeCacheData.MangaBZ) > 0 || len(homeCacheData.DM5) > 0) {
		data := homeCacheData
		homeCacheMu.RUnlock()
		return data
	}
	homeCacheMu.RUnlock()

	fetchCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var mbz, dm5, copyM, pica []MangaSearchResult
	var mbzErr, dm5Err, copyErr, picaErr error

	wg.Add(4)
	go func() {
		defer wg.Done()
		mbz, mbzErr = m.FetchMangaBZRank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		dm5, dm5Err = m.FetchDM5Rank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		copyM, copyErr = m.FetchCopyMangaRank(fetchCtx)
	}()
	go func() {
		defer wg.Done()
		pica, picaErr = m.FetchPicaRank(fetchCtx)
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
	if mbzErr != nil {
		result.MangaBZErr = mbzErr.Error()
	}
	if dm5Err != nil {
		result.DM5Err = dm5Err.Error()
	}
	if copyErr != nil {
		result.CopyMangaErr = copyErr.Error()
	}
	if picaErr != nil {
		result.PicaErr = picaErr.Error()
	}

	homeCacheMu.Lock()
	homeCacheData = result
	homeCacheTime = time.Now()
	homeCacheMu.Unlock()

	return result
}
