package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"OsuCollectionTab/config"
	"OsuCollectionTab/db"
	"OsuCollectionTab/downloader"
	"OsuCollectionTab/utils"
)

func main() {
	workers := flag.Int("workers", 5, "Concurrent download workers")
	delay := flag.Float64("delay", 1.0, "Delay between downloads in seconds")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if !utils.PathExists(cfg.OsuPath) {
		fmt.Printf("Could not find osu! path: %s\n", cfg.OsuPath)
		os.Exit(1)
	}

	// 读取数据库
	osuDBPath := filepath.Join(cfg.OsuPath, "osu!.db")
	collectionDBPath := filepath.Join(cfg.OsuPath, "collection.db")

	osuHashes, err := db.ReadOsuDB(osuDBPath)
	if err != nil {
		fmt.Printf("Failed to read osu!.db: %v\n", err)
		os.Exit(1)
	}

	collectionHashes, err := db.ReadCollectionDB(collectionDBPath)
	if err != nil {
		fmt.Printf("Failed to read collection.db: %v\n", err)
		os.Exit(1)
	}

	// 计算缺失的谱面
	missingHashes := utils.SetDifference(collectionHashes, osuHashes)
	if len(missingHashes) == 0 {
		fmt.Println("No missing beatmaps found!")
		return
	}

	fmt.Printf("Found %d missing beatmaps:\n", len(missingHashes))

	downloadType := utils.PromptDownloadType()

	dl := downloader.NewDownloader(
		filepath.Join(cfg.OsuPath, "Songs"),
		cfg.Proxy,
		*workers,
		time.Duration(*delay*float64(time.Second)),
		cfg.OsuAPIToken,
		downloadType,
	)

	err = dl.DownloadAll(missingHashes)
	if err != nil {
		fmt.Printf("Error downloading beatmaps: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("All missing beatmaps downloaded successfully!")
}