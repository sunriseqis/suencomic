package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"manga-downloader/internal/config"
	"manga-downloader/internal/sources"
)

type TaskStatus string

const (
	StatusPending     TaskStatus = "pending"
	StatusDownloading TaskStatus = "downloading"
	StatusProcessing  TaskStatus = "processing"
	StatusCompleted   TaskStatus = "completed"
	StatusFailed      TaskStatus = "failed"
	StatusPaused      TaskStatus = "paused"
)

type Task struct {
	ID               string     `json:"id"`
	MangaID          string     `json:"manga_id"`
	MangaTitle       string     `json:"manga_title"`
	ChapterID        string     `json:"chapter_id"`
	ChapterTitle     string     `json:"chapter_title"`
	Source           string     `json:"source"`
	ActiveSource     string     `json:"active_source"`
	Format           string     `json:"format"` // "raw", "pdf", "cbz", "epub"
	Status           TaskStatus `json:"status"`
	Progress         float64    `json:"progress"` // 0 - 100
	DownloadedImages int        `json:"downloaded_images"`
	TotalImages      int        `json:"total_images"`
	Error            string     `json:"error,omitempty"`
	Logs             []string   `json:"logs"`
	OutputPath       string     `json:"output_path"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`

	ctx        context.Context    `json:"-"`
	cancelFunc context.CancelFunc `json:"-"`
}

// clone returns a copy safe to hand to readers (SSE marshalling, JSON API).
// Live Task structs are mutated by worker goroutines, so every reader must
// work on a snapshot taken under m.mu instead of the shared pointer.
func (t *Task) clone() Task {
	c := *t
	c.ctx = nil
	c.cancelFunc = nil
	if t.Logs != nil {
		c.Logs = make([]string, len(t.Logs))
		copy(c.Logs, t.Logs)
	}
	return c
}

type CreateTaskRequest struct {
	MangaID      string   `json:"manga_id"`
	MangaTitle   string   `json:"manga_title"`
	Source       string   `json:"source"`
	Format       string   `json:"format"` // "raw", "pdf", "cbz", "epub"
	ChapterIDs   []string `json:"chapter_ids"`
	ChapterNames []string `json:"chapter_names"`
}

type Manager struct {
	tasks       map[string]*Task
	taskOrder   []string
	mu          sync.RWMutex
	sourceMgr   *sources.SourceManager
	taskChan    chan string
	subscribers map[chan *Task]bool
	subMu       sync.RWMutex
	workers     int
	storagePath string
}

var DefaultManager *Manager

func InitManager(sourceMgr *sources.SourceManager) *Manager {
	mgr := &Manager{
		tasks:       make(map[string]*Task),
		taskOrder:   make([]string, 0),
		sourceMgr:   sourceMgr,
		taskChan:    make(chan string, 2000),
		subscribers: make(map[chan *Task]bool),
		workers:     config.Get().MaxConcurrentChapters,
		storagePath: "tasks.json",
	}

	if mgr.workers <= 0 {
		mgr.workers = 3
	}

	mgr.loadTasks()

	// Start worker pool
	for i := 0; i < mgr.workers; i++ {
		go mgr.workerLoop()
	}

	// Auto-queue any pending/downloading tasks from previous run
	mgr.mu.Lock()
	for _, t := range mgr.tasks {
		if t.Status == StatusPending || t.Status == StatusDownloading {
			t.Status = StatusPending
			select {
			case mgr.taskChan <- t.ID:
			default:
			}
		}
	}
	mgr.mu.Unlock()

	DefaultManager = mgr
	return mgr
}

func (m *Manager) loadTasks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.storagePath)
	if err == nil {
		var list []*Task
		if err := json.Unmarshal(data, &list); err == nil {
			for _, t := range list {
				m.tasks[t.ID] = t
				m.taskOrder = append(m.taskOrder, t.ID)
			}
		}
	}
}

func (m *Manager) saveTasks() {
	// Snapshot under lock, marshal outside so long serializations never block
	// workers, and so the data marshalled is a consistent copy.
	m.mu.RLock()
	list := make([]Task, 0, len(m.taskOrder))
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			list = append(list, t.clone())
		}
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(list, "", "  ")
	if err == nil {
		_ = os.WriteFile(m.storagePath, data, 0644)
	}
}

func (m *Manager) Subscribe() chan *Task {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	ch := make(chan *Task, 50)
	m.subscribers[ch] = true
	return ch
}

func (m *Manager) Unsubscribe(ch chan *Task) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	delete(m.subscribers, ch)
	close(ch)
}

// broadcast sends a snapshot of the task to all subscribers. Callers may pass
// the live pointer; the clone is taken under m.mu so readers never observe a
// half-updated struct.
func (m *Manager) broadcast(task *Task) {
	m.mu.RLock()
	c := task.clone()
	m.mu.RUnlock()

	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- &c:
		default:
		}
	}
}

// logAndBroadcast appends a log entry and pushes the update to subscribers.
func (m *Manager) logAndBroadcast(task *Task, msg string) {
	m.mu.Lock()
	task.addLog(msg)
	task.UpdatedAt = time.Now()
	c := task.clone()
	m.mu.Unlock()

	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- &c:
		default:
		}
	}
}

func (m *Manager) AddTasks(req CreateTaskRequest) []*Task {
	m.mu.Lock()

	cfg := config.Get()
	format := req.Format
	if format == "" {
		format = cfg.DefaultFormat
	}
	if format == "" {
		format = "pdf"
	}

	var addedTasks []*Task
	now := time.Now()

	for i, chID := range req.ChapterIDs {
		chTitle := chID
		if i < len(req.ChapterNames) && req.ChapterNames[i] != "" {
			chTitle = req.ChapterNames[i]
		}

		taskID := fmt.Sprintf("%d_%d", now.UnixNano(), i)
		task := &Task{
			ID:           taskID,
			MangaID:      req.MangaID,
			MangaTitle:   req.MangaTitle,
			ChapterID:    chID,
			ChapterTitle: chTitle,
			Source:       req.Source,
			ActiveSource: req.Source,
			Format:       format,
			Status:       StatusPending,
			Progress:     0,
			Logs:         []string{fmt.Sprintf("[%s] 任务已创建，等待下载队列调度", now.Format("15:04:05"))},
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		m.tasks[taskID] = task
		m.taskOrder = append([]string{taskID}, m.taskOrder...) // prepend to top
		addedTasks = append(addedTasks, task)

		// Queue task
		select {
		case m.taskChan <- taskID:
		default:
		}
	}
	m.mu.Unlock()

	m.saveTasks()
	return addedTasks
}

func (m *Manager) GetAllTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	res := make([]*Task, 0, len(m.taskOrder))
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			c := t.clone()
			res = append(res, &c)
		}
	}
	return res
}

func (m *Manager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[id]
	if !ok {
		return nil, false
	}
	c := t.clone()
	return &c, true
}

func (m *Manager) PauseTask(id string) bool {
	m.mu.Lock()

	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return false
	}

	if t.Status == StatusDownloading || t.Status == StatusPending {
		if t.cancelFunc != nil {
			t.cancelFunc()
		}
		t.Status = StatusPaused
		t.addLog("任务已暂停")
		t.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(t)
		m.saveTasks()
		return true
	}
	m.mu.Unlock()
	return false
}

func (m *Manager) ResumeTask(id string) bool {
	m.mu.Lock()

	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return false
	}

	if t.Status == StatusPaused || t.Status == StatusFailed {
		t.Status = StatusPending
		t.Error = ""
		t.addLog("任务已恢复，重新加入队列")
		t.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(t)
		m.saveTasks()

		select {
		case m.taskChan <- id:
		default:
		}
		return true
	}
	m.mu.Unlock()
	return false
}

func (m *Manager) RetryTask(id string) bool {
	return m.ResumeTask(id)
}

func (m *Manager) DeleteTask(id string) bool {
	m.mu.Lock()

	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return false
	}

	if t.cancelFunc != nil {
		t.cancelFunc()
	}

	delete(m.tasks, id)

	newOrder := make([]string, 0, len(m.taskOrder))
	for _, tid := range m.taskOrder {
		if tid != id {
			newOrder = append(newOrder, tid)
		}
	}
	m.taskOrder = newOrder
	m.mu.Unlock()
	m.saveTasks()
	return true
}

func (m *Manager) ClearFinished() {
	m.mu.Lock()

	newOrder := make([]string, 0)
	for _, id := range m.taskOrder {
		if t, ok := m.tasks[id]; ok {
			if t.Status == StatusCompleted {
				delete(m.tasks, id)
			} else {
				newOrder = append(newOrder, id)
			}
		}
	}
	m.taskOrder = newOrder
	m.mu.Unlock()
	m.saveTasks()
}

func (t *Task) addLog(msg string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
	t.Logs = append(t.Logs, entry)
	if len(t.Logs) > 40 {
		t.Logs = t.Logs[len(t.Logs)-40:]
	}
}

func (m *Manager) workerLoop() {
	for taskID := range m.taskChan {
		m.processTask(taskID)
	}
}

func sanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalid {
		name = strings.ReplaceAll(name, ch, "_")
	}
	return strings.TrimSpace(name)
}

func (m *Manager) processTask(taskID string) {
	// Atomically claim the task: the status check and transition to
	// Downloading must happen under one lock, otherwise a double resume could
	// let two workers process the same task concurrently.
	m.mu.Lock()
	task, ok := m.tasks[taskID]
	if !ok || task.Status != StatusPending {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	task.ctx = ctx
	task.cancelFunc = cancel
	task.Status = StatusDownloading
	task.UpdatedAt = time.Now()
	task.addLog(fmt.Sprintf("开始处理下载任务: 《%s》- %s", task.MangaTitle, task.ChapterTitle))
	m.mu.Unlock()
	m.broadcast(task)
	m.saveTasks()

	cfg := config.Get()
	cleanManga := sanitizeFilename(task.MangaTitle)
	cleanChapter := sanitizeFilename(task.ChapterTitle)
	if cleanManga == "" {
		cleanManga = "Unknown_Manga"
	}
	if cleanChapter == "" {
		cleanChapter = "Chapter_" + task.ChapterID
	}

	// Prepare temp cache directory for chapter images
	tempDir := filepath.Join(cfg.DownloadDir, ".cache", cleanManga, cleanChapter)
	_ = os.MkdirAll(tempDir, 0755)

	// Fetch image URLs with automatic fallback
	images, activeSrc, err := m.sourceMgr.GetChapterImagesWithFallback(
		ctx,
		task.MangaTitle,
		task.ChapterTitle,
		task.Source,
		task.MangaID,
		task.ChapterID,
		func(logMsg string) {
			m.logAndBroadcast(task, logMsg)
		},
	)

	if err != nil {
		m.mu.Lock()
		if ctx.Err() != nil {
			task.Status = StatusPaused
			task.addLog("任务已停止")
		} else {
			task.Status = StatusFailed
			task.Error = err.Error()
			task.addLog(fmt.Sprintf("获取图片链接失败: %v", err))
		}
		task.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(task)
		m.saveTasks()
		return
	}

	m.mu.Lock()
	task.ActiveSource = activeSrc
	task.TotalImages = len(images)
	// Count from scratch: a resumed task still carries DownloadedImages from
	// the previous run in tasks.json; existing files are re-counted below.
	task.DownloadedImages = 0
	task.addLog(fmt.Sprintf("解析完成，共 %d 张图片，开始下载数据流...", len(images)))
	m.mu.Unlock()
	m.broadcast(task)
	m.saveTasks()

	// Download images concurrently with resume support
	maxImgWorkers := cfg.MaxConcurrentImages
	if maxImgWorkers <= 0 {
		maxImgWorkers = 5
	}

	// One shared client for the whole task: a fresh transport per image would
	// throw away connection pooling and re-run TLS handshakes 100+ times.
	client := sources.CreateHTTPClient(25 * time.Second)

	sem := make(chan struct{}, maxImgWorkers)
	var dlWg sync.WaitGroup
	var dlErr error
	downloadedPaths := make([]string, len(images))

	for idx, imgURL := range images {
		if ctx.Err() != nil {
			break
		}

		dlWg.Add(1)
		go func(i int, u string) {
			defer dlWg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			// Destination file path
			ext := filepath.Ext(u)
			if ext == "" || len(ext) > 5 || strings.Contains(ext, "?") {
				ext = ".jpg"
			}
			destFile := filepath.Join(tempDir, fmt.Sprintf("%04d%s", i+1, ext))

			// Check existing file (Resume / 断点续传)
			if stat, sErr := os.Stat(destFile); sErr == nil && stat.Size() > 1024 {
				m.mu.Lock()
				downloadedPaths[i] = destFile
				task.DownloadedImages++
				task.Progress = float64(task.DownloadedImages) / float64(task.TotalImages) * 80.0
				task.UpdatedAt = time.Now()
				m.mu.Unlock()
				m.broadcast(task)
				return
			}

			req, rErr := http.NewRequestWithContext(ctx, "GET", u, nil)
			if rErr != nil {
				m.mu.Lock()
				if dlErr == nil {
					dlErr = rErr
				}
				m.mu.Unlock()
				return
			}

			// Set appropriate Referer header
			referer := "https://www.copymanga.tv"
			if strings.Contains(u, "mangabz") || activeSrc == "mangabz" {
				referer = "https://www.mangabz.com/"
			} else if strings.Contains(u, "dm5") || activeSrc == "dm5" {
				referer = "https://www.dm5.com/"
			}

			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
			req.Header.Set("Referer", referer)

			resp, dErr := client.Do(req)
			if dErr != nil {
				m.mu.Lock()
				if dlErr == nil {
					dlErr = dErr
				}
				m.mu.Unlock()
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				m.mu.Lock()
				if dlErr == nil {
					dlErr = fmt.Errorf("image HTTP status %d", resp.StatusCode)
				}
				m.mu.Unlock()
				return
			}

			outF, cErr := os.Create(destFile)
			if cErr != nil {
				m.mu.Lock()
				if dlErr == nil {
					dlErr = cErr
				}
				m.mu.Unlock()
				return
			}
			_, cpErr := io.Copy(outF, resp.Body)
			outF.Close()

			if cpErr != nil {
				m.mu.Lock()
				if dlErr == nil {
					dlErr = cpErr
				}
				m.mu.Unlock()
				return
			}

			m.mu.Lock()
			downloadedPaths[i] = destFile
			task.DownloadedImages++
			task.Progress = float64(task.DownloadedImages) / float64(task.TotalImages) * 80.0
			task.UpdatedAt = time.Now()
			m.mu.Unlock()
			m.broadcast(task)
		}(idx, imgURL)
	}

	dlWg.Wait()

	if ctx.Err() != nil {
		m.mu.Lock()
		task.Status = StatusPaused
		task.addLog("下载被用户暂停")
		task.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(task)
		m.saveTasks()
		return
	}

	m.mu.RLock()
	var validPaths []string
	for _, p := range downloadedPaths {
		if p != "" {
			validPaths = append(validPaths, p)
		}
	}
	total := task.TotalImages
	m.mu.RUnlock()

	// Every requested image must be present — a partial set would produce a
	// corrupted archive reported as complete.
	if len(validPaths) < total {
		m.mu.Lock()
		task.Status = StatusFailed
		if dlErr != nil {
			task.Error = dlErr.Error()
			task.addLog(fmt.Sprintf("图片下载发生错误: %v", dlErr))
		} else {
			task.Error = fmt.Sprintf("downloaded %d/%d images", len(validPaths), total)
			task.addLog(task.Error)
		}
		task.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(task)
		m.saveTasks()
		return
	}

	// Package according to requested format
	m.mu.Lock()
	task.Status = StatusProcessing
	task.Progress = 85.0
	task.addLog(fmt.Sprintf("图片下载完毕，正在打包格式: [%s] ...", strings.ToUpper(task.Format)))
	m.mu.Unlock()
	m.broadcast(task)
	m.saveTasks()

	finalMangaDir := filepath.Join(cfg.DownloadDir, cleanManga)
	_ = os.MkdirAll(finalMangaDir, 0755)

	var outputFinalPath string
	var formatErr error

	switch task.Format {
	case "raw", "images":
		rawChapterDir := filepath.Join(finalMangaDir, cleanChapter)
		_ = os.MkdirAll(rawChapterDir, 0755)
		for i, p := range validPaths {
			ext := filepath.Ext(p)
			target := filepath.Join(rawChapterDir, fmt.Sprintf("%04d%s", i+1, ext))
			data, _ := os.ReadFile(p)
			_ = os.WriteFile(target, data, 0644)
		}
		outputFinalPath = rawChapterDir

	case "cbz":
		cbzPath := filepath.Join(finalMangaDir, cleanChapter+".cbz")
		formatErr = MergeToCBZ(cbzPath, validPaths, task.MangaTitle, task.ChapterTitle)
		outputFinalPath = cbzPath

	case "epub":
		epubPath := filepath.Join(finalMangaDir, cleanChapter+".epub")
		formatErr = MergeToEPUB(epubPath, validPaths, task.MangaTitle, task.ChapterTitle)
		outputFinalPath = epubPath

	case "pdf":
		fallthrough
	default:
		pdfPath := filepath.Join(finalMangaDir, cleanChapter+".pdf")
		formatErr = MergeToPDF(pdfPath, validPaths, fmt.Sprintf("%s - %s", task.MangaTitle, task.ChapterTitle))
		outputFinalPath = pdfPath
	}

	if formatErr != nil {
		m.mu.Lock()
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("打包格式 %s 失败: %v", task.Format, formatErr)
		task.addLog(task.Error)
		task.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.broadcast(task)
		m.saveTasks()
		return
	}

	m.mu.Lock()
	task.OutputPath = outputFinalPath
	task.Progress = 100.0
	task.Status = StatusCompleted
	task.addLog(fmt.Sprintf("✓ 下载完成！输出文件: %s", outputFinalPath))
	task.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.broadcast(task)
	m.saveTasks()
}
