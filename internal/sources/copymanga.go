package sources

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type CopyMangaSource struct {
	baseURLs    []string
	webBaseURLs []string
	aesKey      string
}

func NewCopyMangaSource() *CopyMangaSource {
	return &CopyMangaSource{
		baseURLs: []string{
			"https://api.copymanga.tv",
			"https://api.mangacopy.com",
			"https://api.copymanga.site",
		},
		webBaseURLs: []string{
			"https://www.mangacopy.com",
			"https://www.copymanga.tv",
			"https://www.copymanga.site",
		},
		aesKey: "op0zzpvv.nmn.00p",
	}
}

func (s *CopyMangaSource) ID() string {
	return "copymanga"
}

func (s *CopyMangaSource) Name() string {
	return "CopyManga (拷贝漫画)"
}

func (s *CopyMangaSource) doRequest(ctx context.Context, apiPath string) (*http.Response, error) {
	type result struct {
		resp *http.Response
		err  error
	}

	resChan := make(chan result, len(s.baseURLs))
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	client := CreateHTTPClient(8 * time.Second)

	for _, base := range s.baseURLs {
		go func(baseURL string) {
			fullURL := baseURL + apiPath
			req, err := http.NewRequestWithContext(reqCtx, "GET", fullURL, nil)
			if err != nil {
				resChan <- result{nil, err}
				return
			}

			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
			req.Header.Set("version", "3.0.0")
			req.Header.Set("platform", "3")
			req.Header.Set("source", "copyApp")
			req.Header.Set("webp", "1")
			req.Header.Set("Accept", "application/json, text/plain, */*")

			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == 200 {
				resChan <- result{resp, nil}
				return
			}
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				resChan <- result{nil, err}
			} else {
				resChan <- result{nil, fmt.Errorf("HTTP status %d", resp.StatusCode)}
			}
		}(base)
	}

	var lastErr error
	for i := 0; i < len(s.baseURLs); i++ {
		select {
		case r := <-resChan:
			if r.err == nil && r.resp != nil {
				return r.resp, nil
			}
			lastErr = r.err
		case <-reqCtx.Done():
			return nil, fmt.Errorf("CopyManga request timeout: %w", reqCtx.Err())
		}
	}

	return nil, lastErr
}

func (s *CopyMangaSource) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	resp, err := s.doRequest(ctx, "/api/v3/search/comic?q=%E6%B5%B7%E8%B4%BC%E7%8E%8B&limit=1&offset=0&platform=3")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return time.Since(start), nil
}

type copySearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Results struct {
		List []struct {
			Name     string `json:"name"`
			Alias    string `json:"alias"`
			PathWord string `json:"path_word"`
			Cover    string `json:"cover"`
			Author   []struct {
				Name string `json:"name"`
			} `json:"author"`
		} `json:"list"`
		Total int `json:"total"`
	} `json:"results"`
}

func (s *CopyMangaSource) Search(ctx context.Context, keyword string) ([]MangaSearchResult, error) {
	apiPath := fmt.Sprintf("/api/v3/search/comic?q=%s&limit=20&offset=0&platform=3", url.QueryEscape(keyword))
	resp, err := s.doRequest(ctx, apiPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data copySearchResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var results []MangaSearchResult
	for _, item := range data.Results.List {
		var author string
		if len(item.Author) > 0 {
			author = item.Author[0].Name
		}
		results = append(results, MangaSearchResult{
			ID:         item.PathWord,
			Title:      item.Name,
			Cover:      item.Cover,
			Author:     author,
			Source:     s.ID(),
			SourceName: s.Name(),
		})
	}

	return results, nil
}

// DecryptAES decrypts CopyManga's AES-128-CBC encrypted hex strings
func DecryptAES(encryptedHex string, keyStr string) ([]byte, error) {
	if len(encryptedHex) < 32 {
		return nil, fmt.Errorf("encrypted payload too short")
	}

	iv := []byte(encryptedHex[:16])
	cipherHex := encryptedHex[16:]

	cipherBytes, err := hex.DecodeString(cipherHex)
	if err != nil {
		return nil, fmt.Errorf("hex decode failed: %w", err)
	}

	key := []byte(keyStr)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher failed: %w", err)
	}

	if len(cipherBytes)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(cipherBytes))
	mode.CryptBlocks(decrypted, cipherBytes)

	// PKCS7 Unpadding
	if len(decrypted) == 0 {
		return nil, fmt.Errorf("decrypted data empty")
	}
	padLen := int(decrypted[len(decrypted)-1])
	if padLen > 0 && padLen <= aes.BlockSize && len(decrypted) >= padLen {
		decrypted = decrypted[:len(decrypted)-padLen]
	}

	return decrypted, nil
}

func (s *CopyMangaSource) GetMangaDetail(ctx context.Context, mangaID string) (*MangaDetail, error) {
	client := CreateHTTPClient(15 * time.Second)

	// Strategy 1: Fetch via Web encrypted endpoint (/comicdetail/{mangaID}/chapters)
	for _, webBase := range s.webBaseURLs {
		webURL := fmt.Sprintf("%s/comicdetail/%s/chapters", webBase, mangaID)
		req, err := http.NewRequestWithContext(ctx, "GET", webURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Referer", webBase)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var webResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Results string `json:"results"`
		}

		if err := json.Unmarshal(bodyBytes, &webResp); err == nil && webResp.Results != "" {
			decrypted, dErr := DecryptAES(webResp.Results, s.aesKey)
			if dErr == nil {
				var parsed struct {
					Groups map[string]struct {
						PathWord string `json:"path_word"`
						Name     string `json:"name"`
						Count    int    `json:"count"`
						Chapters []struct {
							ID   string `json:"id"`
							Name string `json:"name"`
							UUID string `json:"uuid"`
						} `json:"chapters"`
					} `json:"groups"`
				}

				if err := json.Unmarshal(decrypted, &parsed); err == nil && len(parsed.Groups) > 0 {
					detail := &MangaDetail{
						ID:         mangaID,
						Title:      mangaID,
						Source:     s.ID(),
						SourceName: s.Name(),
						Chapters:   make([]ChapterInfo, 0),
					}

					// Collect chapters from default or all groups
					order := 1
					for _, group := range parsed.Groups {
						for _, ch := range group.Chapters {
							chUUID := ch.UUID
							if chUUID == "" {
								chUUID = ch.ID
							}
							isTrial := strings.Contains(ch.Name, "试看") || strings.Contains(ch.Name, "試看") || strings.Contains(ch.Name, "预告")
							detail.Chapters = append(detail.Chapters, ChapterInfo{
								ID:      chUUID,
								Title:   ch.Name,
								Order:   order,
								Source:  s.ID(),
								IsTrial: isTrial,
							})
							order++
						}
					}

					if len(detail.Chapters) > 0 {
						detail.Chapters = CleanAndSortChapters(detail.Chapters, s.ID())
						return detail, nil
					}
				}
			}
		}
	}

	// Strategy 2: Fallback to API endpoint (/api/v3/comic/{mangaID}/group/default/chapters)
	apiPath := fmt.Sprintf("/api/v3/comic/%s/group/default/chapters?limit=500&offset=0&platform=3", mangaID)
	resp, err := s.doRequest(ctx, apiPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Results struct {
			List []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				UUID string `json:"uuid"`
			} `json:"list"`
			Total int `json:"total"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	detail := &MangaDetail{
		ID:         mangaID,
		Title:      mangaID,
		Source:     s.ID(),
		SourceName: s.Name(),
		Chapters:   make([]ChapterInfo, 0, len(data.Results.List)),
	}

	for i, ch := range data.Results.List {
		chUUID := ch.UUID
		if chUUID == "" {
			chUUID = ch.ID
		}
		isTrial := strings.Contains(ch.Name, "试看") || strings.Contains(ch.Name, "試看") || strings.Contains(ch.Name, "预告")
		detail.Chapters = append(detail.Chapters, ChapterInfo{
			ID:      chUUID,
			Title:   ch.Name,
			Order:   i + 1,
			Source:  s.ID(),
			IsTrial: isTrial,
		})
	}

	detail.Chapters = CleanAndSortChapters(detail.Chapters, s.ID())

	return detail, nil
}

func (s *CopyMangaSource) GetChapterImages(ctx context.Context, mangaID string, chapterID string) ([]string, error) {
	client := CreateHTTPClient(15 * time.Second)

	// Strategy 1: Web Chapter Page with contentKey
	for _, webBase := range s.webBaseURLs {
		webURL := fmt.Sprintf("%s/comic/%s/chapter/%s", webBase, mangaID, chapterID)
		req, err := http.NewRequestWithContext(ctx, "GET", webURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Referer", webBase)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		html := string(bodyBytes)

		// Look for contentKey in page
		keyRe := regexp.MustCompile(`contentKey\s*=\s*'([^']+)'`)
		keyMatch := keyRe.FindStringSubmatch(html)
		if len(keyMatch) > 1 && keyMatch[1] != "" {
			decrypted, dErr := DecryptAES(keyMatch[1], s.aesKey)
			if dErr == nil {
				var imgList []struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal(decrypted, &imgList); err == nil && len(imgList) > 0 {
					var urls []string
					for _, item := range imgList {
						if item.URL != "" {
							urls = append(urls, item.URL)
						}
					}
					if len(urls) > 0 {
						return urls, nil
					}
				}
			}
		}
	}

	// Strategy 2: API v3 endpoint
	apiPath := fmt.Sprintf("/api/v3/comic/%s/chapter2/%s?platform=3", mangaID, chapterID)
	resp, err := s.doRequest(ctx, apiPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Results struct {
			Chapter struct {
				Contents []struct {
					URL string `json:"url"`
				} `json:"contents"`
			} `json:"chapter"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var images []string
	for _, c := range data.Results.Chapter.Contents {
		if c.URL != "" {
			images = append(images, c.URL)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images found in chapter %s", chapterID)
	}

	return images, nil
}

func init() {
	_ = bytes.NewBuffer
	_ = sort.Slice
}
