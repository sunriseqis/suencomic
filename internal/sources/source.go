package sources

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
	"manga-downloader/internal/config"
)

type MangaSearchResult struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Cover         string `json:"cover"`
	Author        string `json:"author"`
	LatestChapter string `json:"latest_chapter"`
	Source        string `json:"source"`
	SourceName    string `json:"source_name"`
	LatencyMs     int64  `json:"latency_ms"`
}

type ChapterInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Order     int    `json:"order"`
	Source    string `json:"source"`
	IsTrial   bool   `json:"is_trial"`
	Type      string `json:"type"`       // "chapter", "volume", "extra"
	Group     string `json:"group"`      // "连载单话", "单行本", "番外特别篇"
	ExtraData string `json:"extra_data,omitempty"`
}

type MangaDetail struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Cover       string        `json:"cover"`
	Author      string        `json:"author"`
	Description string        `json:"description"`
	Source      string        `json:"source"`
	SourceName  string        `json:"source_name"`
	Chapters    []ChapterInfo `json:"chapters"`
}

type SpeedTestResult struct {
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	LatencyMs  int64  `json:"latency_ms"`
	Available  bool   `json:"available"`
	ErrorMsg   string `json:"error_msg,omitempty"`
	IsFastest  bool   `json:"is_fastest"`
}

type MangaSource interface {
	ID() string
	Name() string
	Search(ctx context.Context, keyword string) ([]MangaSearchResult, error)
	GetMangaDetail(ctx context.Context, mangaID string) (*MangaDetail, error)
	GetChapterImages(ctx context.Context, mangaID string, chapterID string) ([]string, error)
	Ping(ctx context.Context) (time.Duration, error)
}

type SourceManager struct {
	sources map[string]MangaSource
	order   []string
	mu      sync.RWMutex
}

var (
	DefaultManager *SourceManager
)

func InitManager() *SourceManager {
	mgr := &SourceManager{
		sources: make(map[string]MangaSource),
		order:   make([]string, 0),
	}

	// Register the sources
	mgr.Register(NewCopyMangaSource())
	mgr.Register(NewDM5Source())
	mgr.Register(NewMangaBZSource())
	mgr.Register(NewPicaSource())

	DefaultManager = mgr
	return mgr
}

func (m *SourceManager) Register(src MangaSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources[src.ID()] = src
	m.order = append(m.order, src.ID())
}

func (m *SourceManager) GetSource(id string) (MangaSource, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src, ok := m.sources[id]
	return src, ok
}

func (m *SourceManager) ListSources() []MangaSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]MangaSource, 0, len(m.order))
	for _, id := range m.order {
		if src, ok := m.sources[id]; ok {
			res = append(res, src)
		}
	}
	return res
}

func (m *SourceManager) TestAll(ctx context.Context) []SpeedTestResult {
	testCtx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	sources := m.ListSources()
	results := make([]SpeedTestResult, len(sources))
	var wg sync.WaitGroup

	for i, src := range sources {
		wg.Add(1)
		go func(idx int, s MangaSource) {
			defer wg.Done()
			start := time.Now()
			dur, err := s.Ping(testCtx)
			latency := dur.Milliseconds()
			if latency == 0 {
				latency = time.Since(start).Milliseconds()
			}

			res := SpeedTestResult{
				SourceID:   s.ID(),
				SourceName: s.Name(),
				LatencyMs:  latency,
				Available:  err == nil,
			}
			if err != nil {
				res.ErrorMsg = err.Error()
			}
			results[idx] = res
		}(i, src)
	}

	wg.Wait()

	// Find the fastest available source
	var minLatency int64 = 999999
	fastestIdx := -1
	for i, r := range results {
		if r.Available && r.LatencyMs < minLatency {
			minLatency = r.LatencyMs
			fastestIdx = i
		}
	}
	if fastestIdx >= 0 {
		results[fastestIdx].IsFastest = true
	}

	return results
}

func (m *SourceManager) SearchAll(ctx context.Context, keyword string) []MangaSearchResult {
	sources := m.ListSources()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allResults []MangaSearchResult

	// 15-second timeout for aggregated search across all sources
	searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for _, src := range sources {
		wg.Add(1)
		go func(s MangaSource) {
			defer wg.Done()

			start := time.Now()
			res, err := s.Search(searchCtx, keyword)
			latency := time.Since(start).Milliseconds()
			if err == nil && len(res) > 0 {
				for i := range res {
					res[i].LatencyMs = latency
				}
				mu.Lock()
				allResults = append(allResults, res...)
				mu.Unlock()
			}
		}(src)
	}

	// Non-blocking wait: guarantees returning within searchCtx timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-searchCtx.Done():
	}

	return SortSearchResultsByRelevance(allResults, keyword)
}

// GetChapterImagesWithFallback attempts to fetch chapter images from the primary source,
// and automatically falls back to other sources if it encounters 429, timeout, or empty image list.
func (m *SourceManager) GetChapterImagesWithFallback(
	ctx context.Context,
	mangaTitle string,
	chapterTitle string,
	primarySourceID string,
	primaryMangaID string,
	primaryChapterID string,
	logFn func(string),
) ([]string, string, error) {
	if logFn == nil {
		logFn = func(string) {}
	}

	cfg := config.Get()
	primarySrc, ok := m.GetSource(primarySourceID)
	if !ok {
		return nil, "", fmt.Errorf("unknown primary source: %s", primarySourceID)
	}

	logFn(fmt.Sprintf("[%s] 正在请求章节: %s", primarySrc.Name(), chapterTitle))
	images, err := primarySrc.GetChapterImages(ctx, primaryMangaID, primaryChapterID)
	if err == nil && len(images) > 0 {
		return images, primarySourceID, nil
	}

	if !cfg.AutoFallback {
		return nil, primarySourceID, fmt.Errorf("primary source returned no images and auto-fallback is disabled: %w", err)
	}

	logFn(fmt.Sprintf("⚠️ [%s] 章节获取异常 (%v)，正在启动多源智能故障换源 (Smart Auto-Fallback)...", primarySrc.Name(), err))

	// Search other sources
	for _, fallbackSrc := range m.ListSources() {
		if fallbackSrc.ID() == primarySourceID {
			continue
		}

		logFn(fmt.Sprintf("⚡ 正在从备用源 [%s] 检索匹配漫画: 《%s》...", fallbackSrc.Name(), mangaTitle))
		searchResults, sErr := fallbackSrc.Search(ctx, mangaTitle)
		if sErr != nil || len(searchResults) == 0 {
			logFn(fmt.Sprintf("[%s] 未找到对应漫画", fallbackSrc.Name()))
			continue
		}

		// Find closest title match
		var targetMangaID string
		for _, sRes := range searchResults {
			if strings.Contains(sRes.Title, mangaTitle) || strings.Contains(mangaTitle, sRes.Title) {
				targetMangaID = sRes.ID
				break
			}
		}
		if targetMangaID == "" {
			targetMangaID = searchResults[0].ID
		}

		// Fetch manga detail
		detail, dErr := fallbackSrc.GetMangaDetail(ctx, targetMangaID)
		if dErr != nil || detail == nil || len(detail.Chapters) == 0 {
			logFn(fmt.Sprintf("[%s] 获取漫画详情/章节失败", fallbackSrc.Name()))
			continue
		}

		// Match chapter by title or number
		var matchedChapterID string
		cleanTarget := cleanChapterTitle(chapterTitle)
		for _, ch := range detail.Chapters {
			cleanCh := cleanChapterTitle(ch.Title)
			if cleanCh == cleanTarget || strings.Contains(cleanCh, cleanTarget) || strings.Contains(cleanTarget, cleanCh) {
				matchedChapterID = ch.ID
				break
			}
		}

		if matchedChapterID == "" {
			logFn(fmt.Sprintf("[%s] 未在备用源匹配到对应章节: %s", fallbackSrc.Name(), chapterTitle))
			continue
		}

		// Fetch images from fallback source
		fallbackImages, fErr := fallbackSrc.GetChapterImages(ctx, targetMangaID, matchedChapterID)
		if fErr == nil && len(fallbackImages) > 0 {
			logFn(fmt.Sprintf("✓ 成功从备用源 [%s] 获取到 %d 张图片！", fallbackSrc.Name(), len(fallbackImages)))
			return fallbackImages, fallbackSrc.ID(), nil
		}
		logFn(fmt.Sprintf("[%s] 获取图片失败: %v", fallbackSrc.Name(), fErr))
	}

	return nil, primarySourceID, fmt.Errorf("all sources failed to provide images for chapter %s", chapterTitle)
}

func cleanChapterTitle(title string) string {
	t := strings.TrimSpace(title)
	t = strings.ReplaceAll(t, " ", "")
	t = strings.ReplaceAll(t, "第", "")
	t = strings.ReplaceAll(t, "话", "")
	t = strings.ReplaceAll(t, "話", "")
	t = strings.ReplaceAll(t, "章", "")
	t = strings.ReplaceAll(t, "回", "")
	t = strings.ReplaceAll(t, "卷", "")
	return t
}

var (
	proxyRouteMu     sync.RWMutex
	domainNeedsProxy = make(map[string]time.Time)
)

func isDomainProxyOnly(host string) bool {
	proxyRouteMu.RLock()
	defer proxyRouteMu.RUnlock()
	exp, ok := domainNeedsProxy[host]
	return ok && time.Now().Before(exp)
}

func markDomainNeedsProxy(host string) {
	proxyRouteMu.Lock()
	defer proxyRouteMu.Unlock()
	domainNeedsProxy[host] = time.Now().Add(10 * time.Minute)
}

func markDomainDirectOk(host string) {
	proxyRouteMu.Lock()
	defer proxyRouteMu.Unlock()
	delete(domainNeedsProxy, host)
}

// SmartHybridTransport implements prioritized direct connection with automatic proxy fallback
type SmartHybridTransport struct {
	directTransport *http.Transport
	proxyTransport  *http.Transport
	hasProxy        bool
}

func (t *SmartHybridTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.hasProxy || t.proxyTransport == nil {
		return t.directTransport.RoundTrip(req)
	}

	host := req.URL.Hostname()

	// If domain previously failed direct connection, bypass direct probe and route through proxy immediately
	if isDomainProxyOnly(host) {
		return t.proxyTransport.RoundTrip(req)
	}

	// 1. Try Direct Connection First (governed by directTransport dial & header timeouts)
	resp, err := t.directTransport.RoundTrip(req)
	if err == nil && resp != nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusForbidden {
		markDomainDirectOk(host)
		return resp, nil
	}

	// Direct failed or blocked -> remember domain for 10 minutes to avoid probe delays
	markDomainNeedsProxy(host)

	// 2. Seamless fallback to Proxy
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if req.Body != nil && req.GetBody != nil {
		if newBody, bErr := req.GetBody(); bErr == nil {
			req.Body = newBody
		}
	}

	return t.proxyTransport.RoundTrip(req)
}

// CreateHTTPClient creates an HTTP client that prioritizes direct connection and automatically falls back to proxy
func CreateHTTPClient(timeout time.Duration) *http.Client {
	if timeout < 8*time.Second {
		timeout = 10 * time.Second
	}
	cfg := config.Get()
	directDialer := &net.Dialer{
		Timeout:   2500 * time.Millisecond,
		KeepAlive: 30 * time.Second,
	}

	directTransport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext:           directDialer.DialContext,
		TLSHandshakeTimeout:   2500 * time.Millisecond,
		ResponseHeaderTimeout: 4 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		Proxy:                 nil,
	}

	var proxyTransport *http.Transport
	hasProxy := false

	if cfg.Proxy != "" {
		proxyURL, err := url.Parse(cfg.Proxy)
		if err == nil {
			hasProxy = true
			proxyDialer := &net.Dialer{
				Timeout:   4 * time.Second,
				KeepAlive: 30 * time.Second,
			}
			proxyTransport = &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				TLSHandshakeTimeout:   3500 * time.Millisecond,
				ResponseHeaderTimeout: 6 * time.Second,
				DisableKeepAlives:     false,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
			}

			if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
				sDialer, sErr := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxyDialer)
				if sErr == nil {
					proxyTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return sDialer.Dial(network, addr)
					}
				}
			} else {
				proxyTransport.Proxy = http.ProxyURL(proxyURL)
				proxyTransport.DialContext = proxyDialer.DialContext
			}
		}
	}

	if !hasProxy {
		directTransport.Proxy = http.ProxyFromEnvironment
		return &http.Client{
			Transport: directTransport,
			Timeout:   timeout,
		}
	}

	return &http.Client{
		Transport: &SmartHybridTransport{
			directTransport: directTransport,
			proxyTransport:  proxyTransport,
			hasProxy:        hasProxy,
		},
		Timeout: timeout,
	}
}
