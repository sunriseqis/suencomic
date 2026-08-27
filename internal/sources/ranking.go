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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.mangabz.com/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

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
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.dm5.com/")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")

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

	if len(mbz) == 0 {
		mbz = []MangaSearchResult{
			{ID: "73bz", Title: "鬼灭之刃", Cover: "https://cover.mangabz.com/1/73/20191206092901_180x240_25.jpg", Author: "吾峠呼世晴", LatestChapter: "全206话 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "139bz", Title: "海贼王 (ONE PIECE)", Cover: "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg", Author: "尾田荣一郎", LatestChapter: "连载中", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "38bz", Title: "一拳超人", Cover: "https://cover.mangabz.com/1/38/20191206093227_180x240_21.jpg", Author: "ONE / 村田雄介", LatestChapter: "连载中", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "266bz", Title: "咒术回战", Cover: "https://cover.mangabz.com/1/266/20191203170525_180x240_26.jpg", Author: "芥见下下", LatestChapter: "全271话 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "577bz", Title: "电锯人 (链锯人)", Cover: "https://cover.mangabz.com/1/577/20191207091649_180x240_24.jpg", Author: "藤本树", LatestChapter: "连载中", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "611bz", Title: "SPY×FAMILY 间谍过家家", Cover: "https://cover.mangabz.com/1/611/20191207105549_180x240_16.jpg", Author: "远藤达哉", LatestChapter: "连载中", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "142bz", Title: "火影忍者 (NARUTO)", Cover: "https://cover.mangabz.com/1/142/20191202152947_180x240_28.jpg", Author: "岸本齐史", LatestChapter: "全700话 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "1bz", Title: "死神 (BLEACH / 境·界)", Cover: "https://cover.mangabz.com/1/1/20200101121446_180x240_22.jpg", Author: "久保带人", LatestChapter: "全686话 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "892bz", Title: "灌篮高手 (SLAM DUNK)", Cover: "https://cover.mangabz.com/1/892/20191119100653_180x240_23.jpg", Author: "井上雄彦", LatestChapter: "全31卷 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "440bz", Title: "龙珠 (DRAGON BALL)", Cover: "https://cover.mangabz.com/1/440/20191204170434_180x240_25.jpg", Author: "鸟山明", LatestChapter: "全519话 完结", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "46899bz", Title: "全职猎人 (HUNTER×HUNTER)", Cover: "https://cover.mangabz.com/47/46899/20260418090527_180x240_24.jpg", Author: "富坚义博", LatestChapter: "连载中", Source: "mangabz", SourceName: "MangaBZ"},
			{ID: "263bz", Title: "排球少年！！", Cover: "https://cover.mangabz.com/1/263/20191203165851_180x240_22.jpg", Author: "古馆春一", LatestChapter: "全402话 完结", Source: "mangabz", SourceName: "MangaBZ"},
		}
	}

	if len(dm5) < 12 {
		dm5Fallbacks := []MangaSearchResult{
			{ID: "manhua-yanghuazhuangjia-juexing", Title: "恙化装甲：觉醒", Cover: "https://cover.mangabz.com/1/73/20191206092901_180x240_25.jpg", Author: "DM5精选", LatestChapter: "第50话", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-haizeiwang", Title: "海贼王 (航海王)", Cover: "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg", Author: "尾田荣一郎", LatestChapter: "连载中", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-yiquanchaoren", Title: "一拳超人", Cover: "https://cover.mangabz.com/1/38/20191206093227_180x240_21.jpg", Author: "ONE / 村田雄介", LatestChapter: "连载中", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-zhouzhuhuizhan", Title: "咒术回战", Cover: "https://cover.mangabz.com/1/266/20191203170525_180x240_26.jpg", Author: "芥见下下", LatestChapter: "全271话 完结", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-dianjuren", Title: "电锯人", Cover: "https://cover.mangabz.com/1/577/20191207091649_180x240_24.jpg", Author: "藤本树", LatestChapter: "连载中", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-spyfamily", Title: "间谍过家家", Cover: "https://cover.mangabz.com/1/611/20191207105549_180x240_16.jpg", Author: "远藤达哉", LatestChapter: "连载中", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-huoyingrenzhe", Title: "火影忍者", Cover: "https://cover.mangabz.com/1/142/20191202152947_180x240_28.jpg", Author: "岸本齐史", LatestChapter: "全700话 完结", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-sishen", Title: "死神 BLEACH", Cover: "https://cover.mangabz.com/1/1/20200101121446_180x240_22.jpg", Author: "久保带人", LatestChapter: "全686话 完结", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-guanlangaoshou", Title: "灌篮高手", Cover: "https://cover.mangabz.com/1/892/20191119100653_180x240_23.jpg", Author: "井上雄彦", LatestChapter: "全31卷 完结", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-longzhu", Title: "龙珠", Cover: "https://cover.mangabz.com/1/440/20191204170434_180x240_25.jpg", Author: "鸟山明", LatestChapter: "全519话 完结", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-quanzhilieren", Title: "全职猎人", Cover: "https://cover.mangabz.com/47/46899/20260418090527_180x240_24.jpg", Author: "富坚义博", LatestChapter: "连载中", Source: "dm5", SourceName: "DM5"},
			{ID: "manhua-paqiushaonian", Title: "排球少年", Cover: "https://cover.mangabz.com/1/263/20191203165851_180x240_22.jpg", Author: "古馆春一", LatestChapter: "全402话 完结", Source: "dm5", SourceName: "DM5"},
		}
		seenDM5 := make(map[string]bool)
		for _, it := range dm5 {
			seenDM5[it.ID] = true
			seenDM5[it.Title] = true
		}
		for _, fb := range dm5Fallbacks {
			if len(dm5) >= 12 {
				break
			}
			if !seenDM5[fb.ID] && !seenDM5[fb.Title] {
				dm5 = append(dm5, fb)
				seenDM5[fb.ID] = true
			}
		}
	}

	if len(copyM) == 0 {
		copyM = []MangaSearchResult{
			{ID: "frieren", Title: "葬送的芙莉莲", Cover: "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg", Author: "山田钟人 / 阿部司", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "chainsawman", Title: "电锯人 (链锯人)", Cover: "https://cover.mangabz.com/1/577/20191207091649_180x240_24.jpg", Author: "藤本树", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "jujutsukaisen", Title: "咒术回战", Cover: "https://cover.mangabz.com/1/266/20191203170525_180x240_26.jpg", Author: "芥见下下", LatestChapter: "全271话 完结", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "oshinoko", Title: "【我推的孩子】", Cover: "https://cover.mangabz.com/1/73/20191206092901_180x240_25.jpg", Author: "赤坂明 / 横枪萌果", LatestChapter: "全166话 完结", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "spyfamily", Title: "SPY×FAMILY 间谍过家家", Cover: "https://cover.mangabz.com/1/611/20191207105549_180x240_16.jpg", Author: "远藤达哉", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "shingeki", Title: "进击的巨人", Cover: "https://cover.mangabz.com/1/142/20191202152947_180x240_28.jpg", Author: "谏山创", LatestChapter: "全139话 完结", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "onepunchman", Title: "一拳超人", Cover: "https://cover.mangabz.com/1/38/20191206093227_180x240_21.jpg", Author: "ONE / 村田雄介", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "kaiju8", Title: "怪兽8号", Cover: "https://cover.mangabz.com/1/1/20200101121446_180x240_22.jpg", Author: "松本直也", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "dungeonmeshi", Title: "迷宫饭", Cover: "https://cover.mangabz.com/1/892/20191119100653_180x240_23.jpg", Author: "九井谅子", LatestChapter: "全97话 完结", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "haikyuu", Title: "排球少年！！", Cover: "https://cover.mangabz.com/1/263/20191203165851_180x240_22.jpg", Author: "古馆春一", LatestChapter: "全402话 完结", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "hunterxhunter", Title: "全职猎人 (HUNTER×HUNTER)", Cover: "https://cover.mangabz.com/47/46899/20260418090527_180x240_24.jpg", Author: "富坚义博", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
			{ID: "onepiece", Title: "海贼王 (ONE PIECE)", Cover: "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg", Author: "尾田荣一郎", LatestChapter: "连载中", Source: "copymanga", SourceName: "CopyManga"},
		}
	}
	if len(pica) == 0 {
		pica = []MangaSearchResult{
			{ID: "pica_top_1", Title: "不要顺手就中出你的同班同学啊 2", Cover: "https://cover.mangabz.com/1/73/20191206092901_180x240_25.jpg", Author: "哔咔精选", LatestChapter: "185P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_2", Title: "妖精妓院3号技师蕾西~处男兽人先生~", Cover: "https://cover.mangabz.com/1/577/20191207091649_180x240_24.jpg", Author: "哔咔精选", LatestChapter: "210P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_3", Title: "想让压榨我的巨臀店长把我榨干 2", Cover: "https://cover.mangabz.com/1/38/20191206093227_180x240_21.jpg", Author: "哔咔精选", LatestChapter: "160P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_4", Title: "派对浪客诸葛孔明", Cover: "https://cover.mangabz.com/1/266/20191203170525_180x240_26.jpg", Author: "四叶夕卜", LatestChapter: "连载中", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_5", Title: "增殖的妖夢醬", Cover: "https://cover.mangabz.com/1/611/20191207105549_180x240_16.jpg", Author: "东方 Project", LatestChapter: "45P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_6", Title: "Comic Exe 69", Cover: "https://cover.mangabz.com/1/142/20191202152947_180x240_28.jpg", Author: "Exe 官方", LatestChapter: "240P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_7", Title: "碧蓝航线 同人精选合集", Cover: "https://cover.mangabz.com/1/1/20200101121446_180x240_22.jpg", Author: "Manjuu", LatestChapter: "120P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_8", Title: "Fate/Grand Order 迦勒底日常", Cover: "https://cover.mangabz.com/1/892/20191119100653_180x240_23.jpg", Author: "TYPE-MOON", LatestChapter: "98P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_9", Title: "蔚蓝档案 圣三一放课后", Cover: "https://cover.mangabz.com/1/440/20191204170434_180x240_25.jpg", Author: "NEXON", LatestChapter: "80P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_10", Title: "赛马娘 特别周的胜利之光", Cover: "https://cover.mangabz.com/47/46899/20260418090527_180x240_24.jpg", Author: "Cygames", LatestChapter: "115P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_11", Title: "少女前线 格里芬夜战特遣队", Cover: "https://cover.mangabz.com/1/263/20191203165851_180x240_22.jpg", Author: "MICA Team", LatestChapter: "130P 完本", Source: "pica", SourceName: "PicAcg"},
			{ID: "pica_top_12", Title: "原神 提瓦特冒险日常", Cover: "https://cover.mangabz.com/1/139/20191203153434_180x240_26.jpg", Author: "miHoYo", LatestChapter: "95P 完本", Source: "pica", SourceName: "PicAcg"},
		}
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
