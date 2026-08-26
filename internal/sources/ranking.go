package sources

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type HomeRankings struct {
	Shueisha []MangaSearchResult `json:"shueisha"`
	Trending []MangaSearchResult `json:"trending"`
	Classics []MangaSearchResult `json:"classics"`
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
		Cover:         "https://cover.mangabz.com/1/73/20191203153434_180x240_26.jpg",
		Author:        "吾峠呼世晴 (集英社)",
		LatestChapter: "全206话 完结",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "38bz",
		Title:         "一拳超人",
		Cover:         "https://cover.mangabz.com/1/38/20191203153434_180x240_26.jpg",
		Author:        "ONE / 村田雄介 (集英社)",
		LatestChapter: "连载中",
		Source:        "mangabz",
		SourceName:    "MangaBZ",
	},
	{
		ID:            "manhua-zhoushuhuizhan",
		Title:         "咒术回战",
		Cover:         "https://mhfm5hk.cdndm5.com/49/48443/20210323164417_180x240_30.jpg",
		Author:        "芥见下下 (集英社)",
		LatestChapter: "周刊少年Jump",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-dianjianren-tengbenzhi",
		Title:         "电锯人 (链锯人)",
		Cover:         "https://mhfm5hk.cdndm5.com/53/52251/20201214154131_180x240_26.jpg",
		Author:        "藤本树 (集英社)",
		LatestChapter: "少年Jump+ 连载中",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-jiandieguojiajia",
		Title:         "SPY×FAMILY 间谍过家家",
		Cover:         "https://mhfm5hk.cdndm5.com/56/55298/20210629164219_180x240_27.jpg",
		Author:        "远藤达哉 (集英社)",
		LatestChapter: "少年Jump+ 连载中",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-huoyingrenzhe",
		Title:         "火影忍者 (NARUTO)",
		Cover:         "https://mhfm5hk.cdndm5.com/1/433/20190719155618_180x240_21.jpeg",
		Author:        "岸本齐史 (集英社)",
		LatestChapter: "全700话 完结",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-shentan-bleach",
		Title:         "死神 (BLEACH / 境·界)",
		Cover:         "https://mhfm5hk.cdndm5.com/1/434/20190719155618_180x240_21.jpeg",
		Author:        "久保带人 (集英社)",
		LatestChapter: "全686话 完结",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-guanshangaoshou",
		Title:         "灌篮高手 (SLAM DUNK)",
		Cover:         "https://mhfm5hk.cdndm5.com/1/435/20190719155618_180x240_21.jpeg",
		Author:        "井上雄彦 (集英社)",
		LatestChapter: "全31卷 完结",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-longzhu",
		Title:         "龙珠 (DRAGON BALL)",
		Cover:         "https://mhfm5hk.cdndm5.com/1/436/20190719155618_180x240_21.jpeg",
		Author:        "鸟山明 (集英社)",
		LatestChapter: "全519话 完结",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-quanzhilieren",
		Title:         "全职猎人 (HUNTER×HUNTER)",
		Cover:         "https://mhfm5hk.cdndm5.com/1/437/20190719155618_180x240_21.jpeg",
		Author:        "富坚义博 (集英社)",
		LatestChapter: "周刊少年Jump",
		Source:        "dm5",
		SourceName:    "DM5",
	},
	{
		ID:            "manhua-paqiushaonian",
		Title:         "排球少年！！",
		Cover:         "https://mhfm5hk.cdndm5.com/16/15729/20200720162512_180x240_27.jpg",
		Author:        "古馆春一 (集英社)",
		LatestChapter: "全402话 完结",
		Source:        "dm5",
		SourceName:    "DM5",
	},
}

// FetchLiveTrending fetches real-time trending manga from MangaBZ / DM5 ranking pages
func (m *SourceManager) FetchLiveTrending(ctx context.Context) []MangaSearchResult {
	client := CreateHTTPClient(10 * time.Second)
	var results []MangaSearchResult
	seen := make(map[string]bool)

	// 1. Crawl MangaBZ popular list
	req1, err := http.NewRequestWithContext(ctx, "GET", "https://www.mangabz.com/manga-list-0-0-10-p1/", nil)
	if err == nil {
		req1.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req1.Header.Set("Referer", "https://www.mangabz.com/")
		resp1, rErr := client.Do(req1)
		if rErr == nil && resp1.StatusCode == 200 {
			doc, dErr := goquery.NewDocumentFromReader(resp1.Body)
			resp1.Body.Close()
			if dErr == nil {
				doc.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
					tLink := sel.Find(".title a").First()
					title := strings.TrimSpace(tLink.Text())
					href, _ := tLink.Attr("href")
					if title == "" || href == "" {
						return
					}
					id := strings.Trim(href, "/")
					cover, _ := sel.Find("img.mh-cover, img").Attr("src")
					author := strings.TrimSpace(sel.Find(".author").Text())
					latest := strings.TrimSpace(sel.Find(".title a:nth-child(2)").Text())

					if !seen[title] {
						seen[title] = true
						results = append(results, MangaSearchResult{
							ID:            id,
							Title:         title,
							Cover:         cover,
							Author:        author,
							LatestChapter: latest,
							Source:        "mangabz",
							SourceName:    "MangaBZ",
						})
					}
				})
			}
		}
	}

	// 2. Crawl DM5 rank list if needed
	if len(results) < 8 {
		req2, err := http.NewRequestWithContext(ctx, "GET", "https://www.dm5.com/manhua-rank/", nil)
		if err == nil {
			req2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			resp2, rErr := client.Do(req2)
			if rErr == nil && resp2.StatusCode == 200 {
				doc2, dErr := goquery.NewDocumentFromReader(resp2.Body)
				resp2.Body.Close()
				if dErr == nil {
					doc2.Find(".mh-item").Each(func(i int, sel *goquery.Selection) {
						tLink := sel.Find("h2.title a, .title a").First()
						title := strings.TrimSpace(tLink.Text())
						href, _ := tLink.Attr("href")
						if title == "" || href == "" {
							return
						}
						id := strings.Trim(href, "/")
						cover, _ := sel.Find("img").Attr("src")
						author := strings.TrimSpace(sel.Find(".author").Text())

						if !seen[title] {
							seen[title] = true
							results = append(results, MangaSearchResult{
								ID:         id,
								Title:      title,
								Cover:      cover,
								Author:     author,
								Source:     "dm5",
								SourceName: "DM5",
							})
						}
					})
				}
			}
		}
	}

	return results
}

// GetHomeData compiles the homepage ranking and collections
func (m *SourceManager) GetHomeData(ctx context.Context) HomeRankings {
	var wg sync.WaitGroup
	var trending []MangaSearchResult

	wg.Add(1)
	go func() {
		defer wg.Done()
		trending = m.FetchLiveTrending(ctx)
	}()

	wg.Wait()

	return HomeRankings{
		Shueisha: ShueishaCatalog,
		Trending: trending,
		Classics: ShueishaCatalog[0:6],
	}
}

func init() {
	_ = fmt.Sprintf("")
}
