package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type MangaBZSource struct {
	baseURL string
}

func NewMangaBZSource() *MangaBZSource {
	return &MangaBZSource{
		baseURL: "https://www.mangabz.com",
	}
}

func (s *MangaBZSource) ID() string {
	return "mangabz"
}

func (s *MangaBZSource) Name() string {
	return "MangaBZ (漫画BZ)"
}

func (s *MangaBZSource) Ping(ctx context.Context) (time.Duration, error) {
	client := CreateHTTPClient(10 * time.Second)
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	return time.Since(start), nil
}

func (s *MangaBZSource) Search(ctx context.Context, keyword string) ([]MangaSearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?title=%s", s.baseURL, url.QueryEscape(keyword))
	client := CreateHTTPClient(15 * time.Second)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []MangaSearchResult

	doc.Find(".mh-item, .item, .detail-list-item").Each(func(i int, sel *goquery.Selection) {
		titleLink := sel.Find(".title a, h2.title a").First()
		title := strings.TrimSpace(titleLink.Text())
		href, exists := titleLink.Attr("href")
		if !exists || title == "" {
			return
		}

		cleanID := strings.Trim(href, "/")
		cover, _ := sel.Find("img.mh-cover, img").Attr("src")
		author := strings.TrimSpace(sel.Find(".author, p.author").Text())
		latest := strings.TrimSpace(sel.Find(".title a:nth-child(2), .subtitle").Text())

		results = append(results, MangaSearchResult{
			ID:            cleanID,
			Title:         title,
			Cover:         cover,
			Author:        author,
			LatestChapter: latest,
			Source:        s.ID(),
			SourceName:    s.Name(),
		})
	})

	return results, nil
}

func (s *MangaBZSource) GetMangaDetail(ctx context.Context, mangaID string) (*MangaDetail, error) {
	detailURL := fmt.Sprintf("%s/%s/", s.baseURL, strings.Trim(mangaID, "/"))
	client := CreateHTTPClient(15 * time.Second)

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(doc.Find(".detail-info-title, p.detail-info-title, h1.title, .title").First().Text())
	if title == "" {
		title = mangaID
	}
	cover, _ := doc.Find("img.detail-info-cover, .cover img").First().Attr("src")
	author := strings.TrimSpace(doc.Find(".detail-info-tip span:contains('作者') a, .author").First().Text())
	desc := strings.TrimSpace(doc.Find(".detail-info-content, .detail-info-content p").First().Text())

	detail := &MangaDetail{
		ID:          mangaID,
		Title:       title,
		Cover:       cover,
		Author:      author,
		Description: desc,
		Source:      s.ID(),
		SourceName:  s.Name(),
		Chapters:    make([]ChapterInfo, 0),
	}

	order := 1
	doc.Find("a[href*='/m']").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}
		re := regexp.MustCompile(`/m(\d+)/?`)
		m := re.FindStringSubmatch(href)
		if len(m) < 2 {
			return
		}
		chID := m[1]
		chTitle := strings.TrimSpace(sel.Text())
		if chTitle == "" {
			chTitle, _ = sel.Attr("title")
		}
		if chTitle == "" || chTitle == "開始閱讀" || chTitle == "开始阅读" {
			return
		}

		for _, exist := range detail.Chapters {
			if exist.ID == chID {
				return
			}
		}

		detail.Chapters = append(detail.Chapters, ChapterInfo{
			ID:     chID,
			Title:  chTitle,
			Order:  order,
			Source: s.ID(),
		})
		order++
	})

	detail.Chapters = CleanAndSortChapters(detail.Chapters, s.ID())

	return detail, nil
}

func (s *MangaBZSource) GetChapterImages(ctx context.Context, mangaID string, chapterID string) ([]string, error) {
	chapterURL := fmt.Sprintf("%s/m%s/", s.baseURL, strings.TrimPrefix(chapterID, "m"))
	client := CreateHTTPClient(15 * time.Second)

	req, err := http.NewRequestWithContext(ctx, "GET", chapterURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", s.baseURL)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(bodyBytes)

	cidMatch := regexp.MustCompile(`MANGABZ_CID\s*=\s*(\d+)`).FindStringSubmatch(html)
	midMatch := regexp.MustCompile(`MANGABZ_MID\s*=\s*(\d+)`).FindStringSubmatch(html)
	signMatch := regexp.MustCompile(`MANGABZ_VIEWSIGN\s*=\s*"([^"]+)"`).FindStringSubmatch(html)
	dtMatch := regexp.MustCompile(`MANGABZ_VIEWSIGN_DT\s*=\s*"([^"]+)"`).FindStringSubmatch(html)
	countMatch := regexp.MustCompile(`MANGABZ_IMAGE_COUNT\s*=\s*(\d+)`).FindStringSubmatch(html)

	cid := chapterID
	if len(cidMatch) > 1 {
		cid = cidMatch[1]
	}
	mid := "0"
	if len(midMatch) > 1 {
		mid = midMatch[1]
	}
	sign := ""
	if len(signMatch) > 1 {
		sign = signMatch[1]
	}
	dt := ""
	if len(dtMatch) > 1 {
		dt = dtMatch[1]
	}
	totalCount := 1
	if len(countMatch) > 1 {
		if c, cErr := strconv.Atoi(countMatch[1]); cErr == nil && c > 0 {
			totalCount = c
		}
	}

	type pageResult struct {
		page int
		urls []string
	}

	resultsChan := make(chan pageResult, totalCount)
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for page := 1; page <= totalCount; page++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ashxURL := fmt.Sprintf("%s/m%s/chapterimage.ashx?cid=%s&page=%d&key=&_cid=%s&_mid=%s&_dt=%s&_sign=%s",
				s.baseURL, cid, cid, p, cid, mid, url.QueryEscape(dt), sign)

			ashxReq, aErr := http.NewRequestWithContext(ctx, "GET", ashxURL, nil)
			if aErr != nil {
				return
			}
			ashxReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			ashxReq.Header.Set("Referer", chapterURL)

			ashxResp, aErr := client.Do(ashxReq)
			if aErr != nil {
				return
			}
			defer ashxResp.Body.Close()

			ashxBytes, _ := io.ReadAll(ashxResp.Body)
			unpacked := UnpackDeanEdwards(string(ashxBytes))
			pageImgs := ExtractImageURLsFromJS(unpacked)

			resultsChan <- pageResult{page: p, urls: pageImgs}
		}(page)
	}

	wg.Wait()
	close(resultsChan)

	pageMap := make(map[int][]string)
	for res := range resultsChan {
		pageMap[res.page] = res.urls
	}

	var allImages []string
	seen := make(map[string]bool)
	for p := 1; p <= totalCount; p++ {
		if imgs, ok := pageMap[p]; ok {
			for _, img := range imgs {
				if !seen[img] {
					seen[img] = true
					allImages = append(allImages, img)
				}
			}
		}
	}

	if len(allImages) == 0 {
		return nil, fmt.Errorf("no images retrieved for MangaBZ chapter %s", chapterID)
	}

	return allImages, nil
}
