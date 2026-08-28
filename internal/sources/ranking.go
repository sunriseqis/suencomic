package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/internal/config"
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

	// homeFetchMu + homeFetchCh implement a lightweight single-flight so
	// concurrent /api/home requests share one background fetch instead of
	// each hammering the source sites.
	homeFetchMu  sync.Mutex
	homeFetchCh  chan struct{}
	homeFetchTTL = 5 * time.Minute
)

var chromeHeaders = http.Header{
	"User-Agent": []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"},
	"Accept":     []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
}

// fetchPageForScraping fetches an HTML page for scraping with an important
// safeguard: the direct connection in this network is DNS-poisoned, so a
// poisoned/intercepted host can answer with HTTP 200 and a garbage page that
// is NOT the real site. The response is therefore validated against a content
// marker; if it does not match, the domain is marked proxy-only and the fetch
// is retried through the proxy. The returned error (when non-nil) embeds
// diagnostics from both attempts to make remote failures debuggable.
func fetchPageForScraping(ctx context.Context, pageURL string, referer string, marker string) ([]byte, error) {
	setHeaders := func(req *http.Request) {
		req.Header.Set("User-Agent", chromeHeaders.Get("User-Agent"))
		req.Header.Set("Referer", referer)
		req.Header.Set("Accept", chromeHeaders.Get("Accept"))
	}
	tryFetch := func(client *http.Client) (int, []byte, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
		if err != nil {
			return 0, nil, err
		}
		setHeaders(req)
		resp, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return resp.StatusCode, nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 3<<20))
		if err != nil {
			return resp.StatusCode, nil, err
		}
		return resp.StatusCode, body, nil
	}

	status, body, err := tryFetch(CreateHTTPClient(18 * time.Second))
	if err == nil && strings.Contains(string(body), marker) {
		return body, nil
	}
	directInfo := fmt.Sprintf("直连(status=%d bytes=%d err=%v)", status, len(body), err)

	if u, pErr := url.Parse(pageURL); pErr == nil && u.Hostname() != "" {
		markDomainNeedsProxy(u.Hostname())
	}

	cfg := config.Get()
	if cfg.Proxy == "" {
		if err != nil {
			return nil, fmt.Errorf("请求失败: %v (未配置代理，无法重试)", err)
		}
		return nil, fmt.Errorf("页面内容校验失败 (%s, bytes=%d, 未配置代理，无法重试)", directInfo, len(body))
	}

	status2, body2, err2 := tryFetch(CreateProxyOnlyClient(20 * time.Second))
	if err2 == nil && strings.Contains(string(body2), marker) {
		return body2, nil
	}
	proxyInfo := fmt.Sprintf("代理(status=%d bytes=%d err=%v)", status2, len(body2), err2)
	return nil, fmt.Errorf("页面内容校验失败: %s / %s", directInfo, proxyInfo)
}

func parseMangaBZRank(body []byte) ([]MangaSearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
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
		return nil, fmt.Errorf("未在 MangaBZ 页面提取到榜单列表 (bytes=%d, mh_item标记=%d, pageTitle=%q)",
			len(body), strings.Count(string(body), "mh-item"), strings.TrimSpace(doc.Find("title").First().Text()))
	}
	return results, nil
}

// FetchMangaBZRank fetches top 12 real-time popular manga from MangaBZ
func (m *SourceManager) FetchMangaBZRank(ctx context.Context) ([]MangaSearchResult, error) {
	candidateURLs := []string{
		"https://www.mangabz.com/manga-list-0-0-10-p1/", // 人氣排序
		"https://www.mangabz.com/manga-list-0-0-2-p1/",  // 更新時間排序 (备用)
		"https://www.mangabz.com/",                      // 首页 (最后兜底)
	}

	var lastErr error
	for _, pageURL := range candidateURLs {
		body, err := fetchPageForScraping(ctx, pageURL, "https://www.mangabz.com/", "mh-item")
		if err != nil {
			lastErr = err
			continue
		}
		results, pErr := parseMangaBZRank(body)
		if pErr == nil {
			return results, nil
		}
		lastErr = pErr
	}
	return nil, lastErr
}

func parseDM5Rank(body []byte) ([]MangaSearchResult, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	bgURLRegex := regexp.MustCompile(`url\(([^)]+)\)`)
	var results []MangaSearchResult
	seen := make(map[string]bool)

	doc.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
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
		author = strings.ReplaceAll(author, "\n", " ")
		author = strings.ReplaceAll(author, "\r", " ")
		author = strings.ReplaceAll(author, "\t", " ")
		if idx := strings.Index(author, "评分"); idx != -1 {
			author = author[:idx]
		}
		author = strings.TrimPrefix(author, "作者：")
		author = strings.TrimPrefix(author, "作者:")
		author = strings.TrimSpace(author)
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

	if len(results) == 0 {
		return nil, fmt.Errorf("未在 DM5 页面提取到榜单列表 (bytes=%d, mh_item标记=%d, pageTitle=%q)",
			len(body), strings.Count(string(body), "mh-item"), strings.TrimSpace(doc.Find("title").First().Text()))
	}
	return results, nil
}

// FetchDM5Rank fetches top 12 real-time popular manga from DM5 (动漫屋)
func (m *SourceManager) FetchDM5Rank(ctx context.Context) ([]MangaSearchResult, error) {
	candidateURLs := []string{
		"https://www.dm5.com/manhua-rank/",
		"https://www.dm5.com/manhua-list-0-0-10-p1/",
	}

	var lastErr error
	for _, pageURL := range candidateURLs {
		body, err := fetchPageForScraping(ctx, pageURL, "https://www.dm5.com/", "mh-item")
		if err != nil {
			lastErr = err
			continue
		}
		results, pErr := parseDM5Rank(body)
		if pErr == nil {
			return results, nil
		}
		lastErr = pErr
	}
	return nil, lastErr
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
					Name             string                  `json:"name"`
					PathWord         string                  `json:"path_word"`
					Cover            string                  `json:"cover"`
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

// InvalidateHomeCache drops the cached homepage rankings so the next request
// refetches from the sources. Called after proxy/config changes so new
// settings take effect immediately instead of after the cache TTL.
func InvalidateHomeCache() {
	homeCacheMu.Lock()
	homeCacheTime = time.Time{}
	homeCacheMu.Unlock()
}

// GetHomeData compiles the homepage ranking across 3 major sources in real-time
func (m *SourceManager) GetHomeData(ctx context.Context) HomeRankings {
	// Fast path: cache hit (any non-empty ranking counts — a CopyManga-only
	// success is still worth caching so we don't re-hit the sites per request)
	homeCacheMu.RLock()
	if time.Since(homeCacheTime) < homeFetchTTL &&
		(len(homeCacheData.MangaBZ) > 0 || len(homeCacheData.DM5) > 0 || len(homeCacheData.CopyManga) > 0) {
		data := homeCacheData
		homeCacheMu.RUnlock()
		return data
	}
	homeCacheMu.RUnlock()

	// Single-flight: deduplicate concurrent fetches
	homeFetchMu.Lock()
	if homeFetchCh == nil {
		homeFetchCh = make(chan struct{})
		go m.fetchHomeData()
	}
	ch := homeFetchCh
	homeFetchMu.Unlock()

	select {
	case <-ch:
	case <-ctx.Done():
	case <-time.After(25 * time.Second):
	}

	homeCacheMu.RLock()
	defer homeCacheMu.RUnlock()
	return homeCacheData
}

func (m *SourceManager) fetchHomeData() {
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

	homeFetchMu.Lock()
	if ch := homeFetchCh; ch != nil {
		homeFetchCh = nil
		close(ch)
	}
	homeFetchMu.Unlock()
}
