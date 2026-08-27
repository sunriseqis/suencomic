package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"manga-downloader/internal/config"
	"manga-downloader/internal/downloader"
	"manga-downloader/internal/sources"
	"manga-downloader/internal/tracker"
)

func SetupRouter(
	sourceMgr *sources.SourceManager,
	dlMgr *downloader.Manager,
	trackMgr *tracker.Tracker,
	staticFS http.FileSystem,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	api := r.Group("/api")
	{
		// 0. Home & Rankings
		api.GET("/home", func(c *gin.Context) {
			homeData := sourceMgr.GetHomeData(c.Request.Context())
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": homeData})
		})

		// 1. Sources & Speed test
		api.GET("/sources/speedtest", func(c *gin.Context) {
			results := sourceMgr.TestAll(c.Request.Context())
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
		})

		api.GET("/sources", func(c *gin.Context) {
			srcs := sourceMgr.ListSources()
			type srcInfo struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			res := make([]srcInfo, len(srcs))
			for i, s := range srcs {
				res[i] = srcInfo{ID: s.ID(), Name: s.Name()}
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": res})
		})

		// 2. Search
		api.GET("/search", func(c *gin.Context) {
			q := strings.TrimSpace(c.Query("q"))
			if q == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "search keyword cannot be empty"})
				return
			}
			searchCtx, cancel := context.WithTimeout(c.Request.Context(), 5500*time.Millisecond)
			defer cancel()

			sourceID := c.Query("source")
			if sourceID != "" && sourceID != "all" {
				src, ok := sourceMgr.GetSource(sourceID)
				if !ok {
					c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "unknown source"})
					return
				}
				results, err := src.Search(searchCtx, q)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
					return
				}
				results = sources.SortSearchResultsByRelevance(results, q)
				c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
				return
			}

			// Aggregate search across all sources
			results := sourceMgr.SearchAll(searchCtx, q)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
		})

		// 3. Manga Detail & Chapters
		api.GET("/manga/detail", func(c *gin.Context) {
			sourceID := c.Query("source")
			id := c.Query("id")
			if sourceID == "" || id == "" {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "missing source or id parameter"})
				return
			}

			src, ok := sourceMgr.GetSource(sourceID)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "unknown source"})
				return
			}

			detail, err := src.GetMangaDetail(c.Request.Context(), id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"code": 0, "data": detail})
		})

		// 4. Download Tasks
		api.POST("/tasks", func(c *gin.Context) {
			var req downloader.CreateTaskRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": err.Error()})
				return
			}
			if len(req.ChapterIDs) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "no chapters selected"})
				return
			}

			tasks := dlMgr.AddTasks(req)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks, "count": len(tasks)})
		})

		api.GET("/tasks", func(c *gin.Context) {
			tasks := dlMgr.GetAllTasks()
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks})
		})

		api.POST("/tasks/:id/pause", func(c *gin.Context) {
			id := c.Param("id")
			if dlMgr.PauseTask(id) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "task paused"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "task not found or cannot be paused"})
			}
		})

		api.POST("/tasks/:id/resume", func(c *gin.Context) {
			id := c.Param("id")
			if dlMgr.ResumeTask(id) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "task resumed"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "task not found or cannot be resumed"})
			}
		})

		api.POST("/tasks/:id/retry", func(c *gin.Context) {
			id := c.Param("id")
			if dlMgr.RetryTask(id) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "task queued for retry"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "task not found"})
			}
		})

		api.DELETE("/tasks/:id", func(c *gin.Context) {
			id := c.Param("id")
			if dlMgr.DeleteTask(id) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "task deleted"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "task not found"})
			}
		})

		api.DELETE("/tasks", func(c *gin.Context) {
			dlMgr.ClearFinished()
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "finished tasks cleared"})
		})

		// SSE stream for real-time task progress
		api.GET("/tasks/events", func(c *gin.Context) {
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("Transfer-Encoding", "chunked")

			ch := dlMgr.Subscribe()
			defer dlMgr.Unsubscribe(ch)

			notify := c.Request.Context().Done()

			// Send initial keepalive ping
			c.SSEvent("ping", "connected")
			c.Writer.Flush()

			for {
				select {
				case <-notify:
					return
				case task, ok := <-ch:
					if !ok {
						return
					}
					data, err := json.Marshal(task)
					if err == nil {
						c.SSEvent("task_update", string(data))
						c.Writer.Flush()
					}
				case <-time.After(15 * time.Second):
					c.SSEvent("ping", "heartbeat")
					c.Writer.Flush()
				}
			}
		})

		// 5. Subscriptions / Tracker
		api.GET("/subscriptions", func(c *gin.Context) {
			subs := trackMgr.GetAll()
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": subs})
		})

		api.POST("/subscriptions", func(c *gin.Context) {
			var sub tracker.Subscription
			if err := c.ShouldBindJSON(&sub); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": err.Error()})
				return
			}
			res := trackMgr.AddOrUpdate(sub)
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": res})
		})

		api.DELETE("/subscriptions/:id", func(c *gin.Context) {
			id := c.Param("id")
			if trackMgr.Delete(id) {
				c.JSON(http.StatusOK, gin.H{"code": 0, "message": "subscription deleted"})
			} else {
				c.JSON(http.StatusNotFound, gin.H{"code": 1, "error": "subscription not found"})
			}
		})

		api.POST("/subscriptions/check", func(c *gin.Context) {
			count := trackMgr.CheckUpdatesNow()
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": fmt.Sprintf("checked updates, %d new chapters found", count), "new_chapters": count})
		})

		// 6. Settings / Config
		api.GET("/config", func(c *gin.Context) {
			cfg := config.Get()
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
		})

		api.POST("/config", func(c *gin.Context) {
			var newCfg config.Config
			if err := c.ShouldBindJSON(&newCfg); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": err.Error()})
				return
			}
			if err := config.Update(newCfg); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": config.Get()})
		})

		// 7. Downloads file explorer
		api.GET("/downloads/list", func(c *gin.Context) {
			cfg := config.Get()
			type fileEntry struct {
				Name    string `json:"name"`
				Path    string `json:"path"`
				IsDir   bool   `json:"is_dir"`
				Size    int64  `json:"size"`
				ModTime string `json:"mod_time"`
			}

			var files []fileEntry
			_ = filepath.Walk(cfg.DownloadDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil {
					return nil
				}
				if strings.Contains(path, ".cache") {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				rel, _ := filepath.Rel(cfg.DownloadDir, path)
				if rel == "." || rel == "" {
					return nil
				}

				files = append(files, fileEntry{
					Name:    info.Name(),
					Path:    rel,
					IsDir:   info.IsDir(),
					Size:    info.Size(),
					ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
				})
				return nil
			})

			c.JSON(http.StatusOK, gin.H{"code": 0, "data": files, "download_dir": cfg.DownloadDir})
		})

		api.GET("/downloads/file", func(c *gin.Context) {
			relPath := c.Query("path")
			if relPath == "" || strings.Contains(relPath, "..") {
				c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "invalid path"})
				return
			}

			fullPath := filepath.Join(config.Get().DownloadDir, relPath)
			c.File(fullPath)
		})
	}

	// Serve Frontend Static Files
	if staticFS != nil {
		fileServer := http.FileServer(staticFS)
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": "API route not found"})
				return
			}

			// Try to open file in staticFS
			f, err := staticFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}

			// SPA Fallback to index.html
			c.Request.URL.Path = "/"
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}

func init() {
	_ = io.Discard
}
