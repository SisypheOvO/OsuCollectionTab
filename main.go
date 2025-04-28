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

	// osuHashes, err := db.ReadOsuDB(osuDBPath)
	beatmaps, err := db.LoadOsuDBForHash(osuDBPath)
	if err != nil {
		fmt.Printf("Failed to read osu!.db: %v\n", err)
		os.Exit(1)
	}

	osuHashes := make(map[string]struct{}) // 使用空结构体节省内存
	for _, beatmap := range beatmaps {
		if beatmap.Hash != "" { // 确保哈希不为空
			osuHashes[beatmap.Hash] = struct{}{}
		}
	}

	// 2. 读取collection.db中的哈希
	collectionHashes, err := db.ReadCollectionDB(collectionDBPath)
	if err != nil {
		fmt.Printf("Failed to read collection.db: %v\n", err)
		os.Exit(1)
	}

	// 3. 计算缺失的谱面
	missingHashes := make(map[string]struct{})
	for hash := range collectionHashes {
		if _, exists := osuHashes[hash]; !exists {
			missingHashes[hash] = struct{}{}
		}
	}

	if len(missingHashes) == 0 {
		fmt.Println("No missing beatmaps found!")
		return
	}

	fmt.Printf("Found %d missing beatmaps.\n", len(missingHashes))
	fmt.Println("Calculating the count of sets...")

	dl := downloader.NewDownloader(
        filepath.Join(cfg.OsuPath, "Songs"),
        cfg.Proxy,
        *workers,
        time.Duration(*delay*float64(time.Second)),
        cfg.OsuAPIToken,
        "",
    )

    missingHashesBool := make(map[string]bool, len(missingHashes))
    for hash := range missingHashes {
        missingHashesBool[hash] = true
    }

    setIDs := make(map[int64]struct{})
    for hash := range missingHashesBool {
        setID := dl.GetSetIDFromAPI(hash)
        if setID != 0 {
            setIDs[setID] = struct{}{}
        }
    }

    fmt.Printf("They are from %d sets.\n", len(setIDs))

    // 现在才让用户选择下载类型
    downloadType := utils.PromptDownloadType()
    dl.SetDownloadType(downloadType) // 假设downloader有这个方法可以设置type

    err = dl.DownloadAll(setIDs)
    if err != nil {
        fmt.Printf("Error downloading beatmaps: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("All missing beatmaps downloaded successfully!")
}
