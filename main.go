package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"manga-downloader/internal/api"
	"manga-downloader/internal/config"
	"manga-downloader/internal/downloader"
	"manga-downloader/internal/sources"
	"manga-downloader/internal/tracker"
)

//go:embed web/dist/*
var embeddedWeb embed.FS

func getFileSystem() http.FileSystem {
	sub, err := fs.Sub(embeddedWeb, "web/dist")
	if err != nil {
		return nil
	}
	return http.FS(sub)
}

func main() {
	portFlag := flag.Int("port", 0, "Server port")
	flag.Parse()

	fmt.Println("==================================================")
	fmt.Println("       SUENCOMIC // 多源漫画下载平台")
	fmt.Println("  Sources: CopyManga | DM5 | MangaBZ")
	fmt.Println("  Speed Benchmarking & Intelligent Auto-Fallback")
	fmt.Println("==================================================")

	// 1. Initialize Configuration
	cfg := config.Init()
	if *portFlag > 0 {
		cfg.Port = *portFlag
	}
	fmt.Printf("[CONFIG] Output Directory: %s\n", cfg.DownloadDir)
	if cfg.Proxy != "" {
		fmt.Printf("[CONFIG] Global Proxy: %s\n", cfg.Proxy)
	}

	// 2. Initialize Source Manager
	sourceMgr := sources.InitManager()
	fmt.Println("[SOURCES] Registered CopyManga, DM5, MangaBZ scrapers.")

	// 3. Initialize Downloader Manager
	dlMgr := downloader.InitManager(sourceMgr)

	// 4. Initialize Tracker / Subscription
	trackMgr := tracker.InitTracker(sourceMgr, dlMgr)

	// 5. Setup Web & API Router
	staticFS := getFileSystem()
	router := api.SetupRouter(sourceMgr, dlMgr, trackMgr, staticFS)

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("[SERVER] SUENCOMIC is running at: http://localhost:%d\n", cfg.Port)
	fmt.Println("==================================================")

	if err := router.Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed to start: %v\n", err)
		os.Exit(1)
	}
}
