package tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"manga-downloader/internal/config"
	"manga-downloader/internal/downloader"
	"manga-downloader/internal/sources"
)

type Subscription struct {
	ID               string    `json:"id"`
	MangaID          string    `json:"manga_id"`
	MangaTitle       string    `json:"manga_title"`
	Source           string    `json:"source"`
	Format           string    `json:"format"`
	LastChapterTitle string    `json:"last_chapter_title"`
	LastChapterID    string    `json:"last_chapter_id"`
	AutoDownload     bool      `json:"auto_download"`
	LastCheckedAt    time.Time `json:"last_checked_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type Tracker struct {
	subs        map[string]*Subscription
	mu          sync.RWMutex
	sourceMgr   *sources.SourceManager
	dlMgr       *downloader.Manager
	storagePath string
	stopChan    chan struct{}
}

var DefaultTracker *Tracker

func InitTracker(sourceMgr *sources.SourceManager, dlMgr *downloader.Manager) *Tracker {
	t := &Tracker{
		subs:        make(map[string]*Subscription),
		sourceMgr:   sourceMgr,
		dlMgr:       dlMgr,
		storagePath: "subscriptions.json",
		stopChan:    make(chan struct{}),
	}

	t.load()
	go t.startScheduler()

	DefaultTracker = t
	return t
}

func (t *Tracker) load() {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := os.ReadFile(t.storagePath)
	if err == nil {
		var list []*Subscription
		if err := json.Unmarshal(data, &list); err == nil {
			for _, s := range list {
				t.subs[s.ID] = s
			}
		}
	}
}

func (t *Tracker) save() error {
	list := make([]*Subscription, 0, len(t.subs))
	for _, s := range t.subs {
		list = append(list, s)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.storagePath, data, 0644)
}

func (t *Tracker) GetAll() []*Subscription {
	t.mu.RLock()
	defer t.mu.RUnlock()

	res := make([]*Subscription, 0, len(t.subs))
	for _, s := range t.subs {
		res = append(res, s)
	}
	return res
}

func (t *Tracker) AddOrUpdate(sub Subscription) *Subscription {
	t.mu.Lock()
	defer t.mu.Unlock()

	if sub.ID == "" {
		sub.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if sub.CreatedAt.IsZero() {
		sub.CreatedAt = time.Now()
	}
	if sub.Format == "" {
		sub.Format = config.Get().DefaultFormat
	}

	t.subs[sub.ID] = &sub
	_ = t.save()
	return &sub
}

func (t *Tracker) Delete(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.subs[id]; ok {
		delete(t.subs, id)
		_ = t.save()
		return true
	}
	return false
}

func (t *Tracker) CheckUpdatesNow() int {
	subs := t.GetAll()
	totalNewChapters := 0

	for _, sub := range subs {
		newCount := t.checkSingleManga(sub)
		totalNewChapters += newCount
	}

	return totalNewChapters
}

func (t *Tracker) checkSingleManga(sub *Subscription) int {
	src, ok := t.sourceMgr.GetSource(sub.Source)
	if !ok {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	detail, err := src.GetMangaDetail(ctx, sub.MangaID)
	if err != nil || detail == nil || len(detail.Chapters) == 0 {
		return 0
	}

	sub.LastCheckedAt = time.Now()

	// Check if there are new chapters compared to LastChapterID or LastChapterTitle
	var newChapterIDs []string
	var newChapterNames []string
	foundLast := false

	if sub.LastChapterID == "" {
		// First time tracking, mark latest chapter
		lastCh := detail.Chapters[len(detail.Chapters)-1]
		sub.LastChapterID = lastCh.ID
		sub.LastChapterTitle = lastCh.Title
		t.AddOrUpdate(*sub)
		return 0
	}

	for _, ch := range detail.Chapters {
		if foundLast {
			newChapterIDs = append(newChapterIDs, ch.ID)
			newChapterNames = append(newChapterNames, ch.Title)
		} else if ch.ID == sub.LastChapterID || ch.Title == sub.LastChapterTitle {
			foundLast = true
		}
	}

	if len(newChapterIDs) > 0 {
		// Update latest chapter in subscription
		sub.LastChapterID = newChapterIDs[len(newChapterIDs)-1]
		sub.LastChapterTitle = newChapterNames[len(newChapterNames)-1]
		t.AddOrUpdate(*sub)

		if sub.AutoDownload {
			t.dlMgr.AddTasks(downloader.CreateTaskRequest{
				MangaID:      sub.MangaID,
				MangaTitle:   sub.MangaTitle,
				Source:       sub.Source,
				Format:       sub.Format,
				ChapterIDs:   newChapterIDs,
				ChapterNames: newChapterNames,
			})
		}
	}

	return len(newChapterIDs)
}

func (t *Tracker) startScheduler() {
	// Tick every minute and compare against the configured interval so that
	// changes to check_interval_minutes take effect without a restart.
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	var lastCheck time.Time
	for {
		select {
		case <-t.stopChan:
			return
		case <-ticker.C:
			interval := config.Get().CheckIntervalMinutes
			if interval <= 0 {
				interval = 60
			}
			if !lastCheck.IsZero() && time.Since(lastCheck) < time.Duration(interval)*time.Minute {
				continue
			}
			lastCheck = time.Now()
			t.CheckUpdatesNow()
		}
	}
}
