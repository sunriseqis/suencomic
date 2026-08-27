package sources

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"manga-downloader/internal/config"
)

const (
	PicaAPIKey    = "C69BAF41DA5ABD1FFEDC6D2FEA56B"
	PicaSecretKey = "~d}$Q7$eIni=V)9\\RK/P.RM4;9"
	PicaBaseURL   = "https://picaapi.picacomic.com"
)

type PicaSource struct {
	baseURL   string
	apiKey    string
	secretKey string
	token     string
	tokenExp  time.Time
	mu        sync.RWMutex
}

func NewPicaSource() *PicaSource {
	return &PicaSource{
		baseURL:   PicaBaseURL,
		apiKey:    PicaAPIKey,
		secretKey: PicaSecretKey,
	}
}

func (s *PicaSource) ID() string {
	return "pica"
}

func (s *PicaSource) Name() string {
	return "PicAcg (哔咔漫画)"
}

func generateNonce() string {
	chars := "0123456789abcdef"
	b := make([]byte, 32)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func (s *PicaSource) generateSignature(rawPath, timestamp, nonce, method string) string {
	// Raw data: (path + time + nonce + method + apiKey) in lowercase
	raw := rawPath + timestamp + nonce + method + s.apiKey
	raw = strings.ToLower(raw)

	mac := hmac.New(sha256.New, []byte(s.secretKey))
	mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *PicaSource) setHeaders(req *http.Request, rawPath string) {
	now := strconvFormatInt(time.Now().Unix())
	nonce := generateNonce()
	method := strings.ToUpper(req.Method)

	sig := s.generateSignature(rawPath, now, nonce, method)

	req.Header.Set("api-key", s.apiKey)
	req.Header.Set("accept", "application/vnd.picacomic.com.v1+json")
	req.Header.Set("app-channel", "2")
	req.Header.Set("time", now)
	req.Header.Set("nonce", nonce)
	req.Header.Set("signature", sig)
	req.Header.Set("app-version", "2.2.1.3.3.4")
	req.Header.Set("app-platform", "android")
	req.Header.Set("app-build-version", "45")
	req.Header.Set("app-uuid", "defaultUuid")
	req.Header.Set("image-quality", "original")
	req.Header.Set("User-Agent", "okhttp/3.8.1")

	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token != "" {
		req.Header.Set("authorization", token)
	}
}

func strconvFormatInt(i int64) string {
	return fmt.Sprintf("%d", i)
}

func (s *PicaSource) ensureLogin(ctx context.Context) error {
	s.mu.RLock()
	if s.token != "" && time.Now().Before(s.tokenExp) {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	cfg := config.Get()
	if cfg.PicaAccount == "" || cfg.PicaPassword == "" {
		return fmt.Errorf("请在系统设置中配置哔咔账号与密码 (Pica Account & Password)")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	loginURL := s.baseURL + "/auth/sign-in"
	bodyMap := map[string]string{
		"email":    cfg.PicaAccount,
		"password": cfg.PicaPassword,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	s.setHeaders(req, "auth/sign-in")

	client := CreateHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("哔咔登录连接失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var loginResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBytes, &loginResp); err != nil {
		return fmt.Errorf("解析登录响应失败: %w", err)
	}

	if loginResp.Code != 200 || loginResp.Data.Token == "" {
		return fmt.Errorf("哔咔登录失败: %s (code %d)", loginResp.Message, loginResp.Code)
	}

	s.token = loginResp.Data.Token
	s.tokenExp = time.Now().Add(24 * time.Hour)
	return nil
}

func (s *PicaSource) doRequest(ctx context.Context, method, rawPath string, body io.Reader) (*http.Response, error) {
	cleanPath := strings.TrimPrefix(rawPath, "/")
	fullURL := s.baseURL + "/" + cleanPath
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req, cleanPath)

	client := CreateHTTPClient(8 * time.Second)
	return client.Do(req)
}

func (s *PicaSource) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	client := CreateHTTPClient(3 * time.Second)

	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/categories", nil)
	if err != nil {
		return 0, err
	}
	s.setHeaders(req, "categories")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 401 {
		return 0, fmt.Errorf("HTTP status %d", resp.StatusCode)
	}
	return time.Since(start), nil
}

func (s *PicaSource) Search(ctx context.Context, keyword string) ([]MangaSearchResult, error) {
	if err := s.ensureLogin(ctx); err != nil {
		return nil, err
	}

	searchURL := fmt.Sprintf("%s/comics/advanced-search?page=1", s.baseURL)
	payload := map[string]interface{}{
		"keyword": keyword,
		"sort":    "dd",
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", searchURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	s.setHeaders(req, "comics/advanced-search")

	client := CreateHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var searchResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Comics struct {
				Docs []struct {
					ID     string `json:"_id"`
					Title  string `json:"title"`
					Author string `json:"author"`
					Thumb  struct {
						FileServer string `json:"fileServer"`
						Path       string `json:"path"`
					} `json:"thumb"`
					PagesCount    int    `json:"pagesCount"`
					EpsCount      int    `json:"epsCount"`
					Finished      bool   `json:"finished"`
					LatestChapter string `json:"latestChapter"`
				} `json:"docs"`
			} `json:"comics"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
		return nil, err
	}

	var results []MangaSearchResult
	for _, doc := range searchResp.Data.Comics.Docs {
		cover := ""
		if doc.Thumb.FileServer != "" && doc.Thumb.Path != "" {
			cover = doc.Thumb.FileServer + "/static/" + doc.Thumb.Path
		}

		latest := fmt.Sprintf("共 %d 话", doc.EpsCount)
		if doc.Finished {
			latest += " (完结)"
		}

		results = append(results, MangaSearchResult{
			ID:            doc.ID,
			Title:         doc.Title,
			Cover:         cover,
			Author:        doc.Author,
			LatestChapter: latest,
			Source:        s.ID(),
			SourceName:    s.Name(),
		})
	}

	return results, nil
}

func (s *PicaSource) GetMangaDetail(ctx context.Context, mangaID string) (*MangaDetail, error) {
	if err := s.ensureLogin(ctx); err != nil {
		return nil, err
	}

	client := CreateHTTPClient(15 * time.Second)

	// 1. Fetch Comic Meta
	detailURL := fmt.Sprintf("%s/comics/%s", s.baseURL, mangaID)
	req1, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil, err
	}
	s.setHeaders(req1, "comics/"+mangaID)

	resp1, err := client.Do(req1)
	if err != nil {
		return nil, err
	}
	defer resp1.Body.Close()

	body1, _ := io.ReadAll(resp1.Body)
	var comicResp struct {
		Code int `json:"code"`
		Data struct {
			Comic struct {
				ID          string `json:"_id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Author      string `json:"author"`
				Thumb       struct {
					FileServer string `json:"fileServer"`
					Path       string `json:"path"`
				} `json:"thumb"`
			} `json:"comic"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body1, &comicResp)

	comic := comicResp.Data.Comic
	cover := ""
	if comic.Thumb.FileServer != "" && comic.Thumb.Path != "" {
		cover = comic.Thumb.FileServer + "/static/" + comic.Thumb.Path
	}

	detail := &MangaDetail{
		ID:          mangaID,
		Title:       comic.Title,
		Cover:       cover,
		Author:      comic.Author,
		Description: comic.Description,
		Source:      s.ID(),
		SourceName:  s.Name(),
		Chapters:    make([]ChapterInfo, 0),
	}

	// 2. Fetch all Episodes
	page := 1
	for {
		epsURL := fmt.Sprintf("%s/comics/%s/eps?page=%d", s.baseURL, mangaID, page)
		reqEps, eErr := http.NewRequestWithContext(ctx, "GET", epsURL, nil)
		if eErr != nil {
			break
		}
		s.setHeaders(reqEps, fmt.Sprintf("comics/%s/eps", mangaID))

		respEps, eErr := client.Do(reqEps)
		if eErr != nil {
			break
		}
		epsBody, _ := io.ReadAll(respEps.Body)
		respEps.Body.Close()

		var epsData struct {
			Code int `json:"code"`
			Data struct {
				Eps struct {
					Docs []struct {
						ID    string `json:"_id"`
						Order int    `json:"order"`
						Title string `json:"title"`
					} `json:"docs"`
					Total int `json:"total"`
					Pages int `json:"pages"`
				} `json:"eps"`
			} `json:"data"`
		}

		if err := json.Unmarshal(epsBody, &epsData); err != nil || len(epsData.Data.Eps.Docs) == 0 {
			break
		}

		for _, ep := range epsData.Data.Eps.Docs {
			detail.Chapters = append(detail.Chapters, ChapterInfo{
				ID:        strconvFormatInt(int64(ep.Order)),
				Title:     ep.Title,
				Order:     ep.Order,
				Source:    s.ID(),
				ExtraData: ep.ID,
			})
		}

		if page >= epsData.Data.Eps.Pages {
			break
		}
		page++
	}

	detail.Chapters = CleanAndSortChapters(detail.Chapters, s.ID())
	return detail, nil
}

func (s *PicaSource) GetChapterImages(ctx context.Context, mangaID string, chapterID string) ([]string, error) {
	if err := s.ensureLogin(ctx); err != nil {
		return nil, err
	}

	client := CreateHTTPClient(20 * time.Second)
	var images []string
	page := 1

	for {
		pageURL := fmt.Sprintf("%s/comics/%s/order/%s/pages?page=%d", s.baseURL, mangaID, chapterID, page)
		req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
		if err != nil {
			break
		}
		s.setHeaders(req, fmt.Sprintf("comics/%s/order/%s/pages", mangaID, chapterID))

		resp, err := client.Do(req)
		if err != nil {
			break
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var pageData struct {
			Code int `json:"code"`
			Data struct {
				Pages struct {
					Docs []struct {
						Media struct {
							FileServer string `json:"fileServer"`
							Path       string `json:"path"`
						} `json:"media"`
					} `json:"docs"`
					Pages int `json:"pages"`
					Total int `json:"total"`
				} `json:"pages"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &pageData); err != nil || len(pageData.Data.Pages.Docs) == 0 {
			break
		}

		for _, doc := range pageData.Data.Pages.Docs {
			if doc.Media.FileServer != "" && doc.Media.Path != "" {
				imgURL := doc.Media.FileServer + "/static/" + doc.Media.Path
				images = append(images, imgURL)
			}
		}

		if page >= pageData.Data.Pages.Pages {
			break
		}
		page++
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images retrieved for PicAcg chapter %s", chapterID)
	}

	return images, nil
}

func init() {
	_ = url.QueryEscape
}
