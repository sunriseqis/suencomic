package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type DM5Source struct {
	baseURL string
}

func NewDM5Source() *DM5Source {
	return &DM5Source{
		baseURL: "https://www.dm5.com",
	}
}

func (s *DM5Source) ID() string {
	return "dm5"
}

func (s *DM5Source) Name() string {
	return "DM5 (动漫屋)"
}

func (s *DM5Source) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	client := CreateHTTPClient(10 * time.Second)
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

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("status code %d", resp.StatusCode)
	}
	return time.Since(start), nil
}

func (s *DM5Source) Search(ctx context.Context, keyword string) ([]MangaSearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?title=%s&language=1", s.baseURL, url.QueryEscape(keyword))
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
	seen := make(map[string]bool)

	// Selector matches standard items as well as top highlighted banners in DM5 search
	doc.Find(".mh-item, .banner_detail_form, .item, .banner_detail, .search-result-item, .mh-card-wrap, div.title, .search-result").Each(func(i int, sel *goquery.Selection) {
		titleLink := sel.Find(".title a, h2.title a, dt a, p.title a").First()
		title := strings.TrimSpace(titleLink.Text())
		href, exists := titleLink.Attr("href")
		if !exists || title == "" {
			return
		}

		cleanID := strings.Trim(href, "/")
		if !strings.HasPrefix(cleanID, "manhua-") && !strings.Contains(cleanID, "bz") {
			if !strings.Contains(href, "/manhua-") {
				return
			}
		}

		if seen[cleanID] {
			return
		}
		seen[cleanID] = true

		cover, _ := sel.Find("img").Attr("src")
		if cover == "" {
			cover, _ = sel.Find(".cover img, img.mh-cover").Attr("src")
		}
		author := strings.TrimSpace(sel.Find(".author, p.author, .info p:contains('作者'), p.subtitle").Text())
		author = strings.TrimPrefix(author, "作者：")
		author = strings.TrimPrefix(author, "作者:")

		latest := strings.TrimSpace(sel.Find(".chapter, .subtitle, p.subtitle, span:contains('最新') + a").Text())

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

func (s *DM5Source) GetMangaDetail(ctx context.Context, mangaID string) (*MangaDetail, error) {
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

	// Try specific title selectors in priority order — the bare "h1" fallback
	// must stay last because DM5 pages embed a login dialog whose <h1>登录</h1>
	// would otherwise become the manga title. The info block mixes the title
	// with rating spans, so clone and strip them before reading the text.
	titleText := func(sel string) string {
		s := doc.Find(sel).First().Clone()
		s.Find(".right, .score").Remove()
		return s.Text()
	}
	title := pickTitle([]string{
		titleText("h1.title"),
		titleText(".banner_detail_form .info p.title"),
		titleText(".banner_detail h1"),
		titleText(".info h1"),
		titleText("h1"),
	})
	if title == "" {
		// Fall back to the <title> tag: "朋友登录漫画_14已完结_在线漫画_动漫屋"
		if t := strings.TrimSpace(doc.Find("title").First().Text()); t != "" {
			part := strings.SplitN(t, "_", 2)[0]
			part = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(part), "漫画"))
			title = part
		}
	}
	if title == "" {
		title = mangaID
	}
	cover, _ := doc.Find(".banner_detail img, .cover img, .info img").First().Attr("src")
	author := strings.TrimSpace(doc.Find(".author, .info p:contains('作者'), .detail-info-tip span:contains('作者') a").First().Text())
	author = strings.TrimPrefix(author, "作者：")
	author = strings.TrimPrefix(author, "作者:")
	desc := strings.TrimSpace(doc.Find(".content, .info p.content, #intro, .detail-info-content").First().Text())

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

	// Target only genuine chapter list containers to avoid picking up footer/sidebar recommendation links
	chSelector := "#chapterlistload a[href*='/m'], .detail-list-select a[href*='/m'], .detail-list-form-con a[href*='/m'], .view-win-list a[href*='/m'], #danhaobian a[href*='/m'], #manhua-chapter-list a[href*='/m'], .chapter-list a[href*='/m']"
	chElements := doc.Find(chSelector)
	if chElements.Length() == 0 {
		// Fallback only to .detail-list containers
		chElements = doc.Find(".detail-list a[href*='/m']")
	}

	order := 1
	chElements.Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}
		// Match /m12345/ or /m12345-p1/
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

func (s *DM5Source) GetChapterImages(ctx context.Context, mangaID string, chapterID string) ([]string, error) {
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

	// Extract DM5 metadata
	cidRe := regexp.MustCompile(`DM5_CID\s*=\s*(\d+)`)
	midRe := regexp.MustCompile(`DM5_MID\s*=\s*(\d+)`)
	countRe := regexp.MustCompile(`DM5_IMAGE_COUNT\s*=\s*(\d+)`)
	viewSignRe := regexp.MustCompile(`DM5_VIEWSIGN\s*=\s*["']([^"']+)["']`)
	viewSignDtRe := regexp.MustCompile(`DM5_VIEWSIGN_DT\s*=\s*["']([^"']+)["']`)

	cidMatch := cidRe.FindStringSubmatch(html)
	midMatch := midRe.FindStringSubmatch(html)
	countMatch := countRe.FindStringSubmatch(html)
	viewSignMatch := viewSignRe.FindStringSubmatch(html)
	viewSignDtMatch := viewSignDtRe.FindStringSubmatch(html)

	if len(cidMatch) < 2 || len(countMatch) < 2 {
		images := ExtractImagesFromJS(html)
		if len(images) > 0 {
			return images, nil
		}
		return nil, fmt.Errorf("failed to parse DM5 chapter metadata for %s", chapterID)
	}

	cid := cidMatch[1]
	mid := ""
	if len(midMatch) > 1 {
		mid = midMatch[1]
	}
	var totalPages int
	fmt.Sscanf(countMatch[1], "%d", &totalPages)
	if totalPages <= 0 {
		totalPages = 1
	}

	viewSign := ""
	if len(viewSignMatch) > 1 {
		viewSign = viewSignMatch[1]
	}
	viewSignDt := ""
	if len(viewSignDtMatch) > 1 {
		viewSignDt = viewSignDtMatch[1]
	}

	var images []string
	failedPages := 0
	for page := 1; page <= totalPages; page++ {
		ashxURL := fmt.Sprintf("%s/m%s/chapterimage.ashx?cid=%s&page=%d&key=&_type=1&_cid=%s&_mid=%s&_dt=%s&_sign=%s",
			s.baseURL, cid, cid, page, cid, mid, url.QueryEscape(viewSignDt), viewSign)

		ashxReq, aErr := http.NewRequestWithContext(ctx, "GET", ashxURL, nil)
		if aErr != nil {
			failedPages++
			continue
		}
		ashxReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		ashxReq.Header.Set("Referer", chapterURL)

		ashxResp, aErr := client.Do(ashxReq)
		if aErr != nil {
			failedPages++
			continue
		}

		ashxBody, _ := io.ReadAll(ashxResp.Body)
		ashxResp.Body.Close()

		jsCode := string(ashxBody)
		pageImages := ExtractImagesFromJS(jsCode)
		if len(pageImages) > 0 {
			images = append(images, pageImages...)
		} else {
			failedPages++
		}
	}

	if len(images) == 0 {
		images = ExtractImagesFromJS(html)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images retrieved for DM5 chapter %s", chapterID)
	}

	// Never return a silently incomplete chapter: a missing middle page would
	// otherwise produce a corrupted archive reported as 100% complete.
	if failedPages > 0 {
		return nil, fmt.Errorf("DM5 chapter %s incomplete: %d/%d pages failed to load", chapterID, failedPages, totalPages)
	}

	return images, nil
}
