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
	// 解析命令行参数
	workers := flag.Int("workers", 5, "并发工作线程数")
	delay := flag.Float64("delay", 1.0, "下载间隔秒数")
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 检查 osu! 路径
	if !utils.PathExists(cfg.OsuPath) {
		fmt.Printf("osu! 路径不存在: %s\n", cfg.OsuPath)
		os.Exit(1)
	}

	// 读取数据库
	osuDBPath := filepath.Join(cfg.OsuPath, "osu!.db")
	collectionDBPath := filepath.Join(cfg.OsuPath, "collection.db")

	osuHashes, err := db.ReadOsuDB(osuDBPath)
	if err != nil {
		fmt.Printf("读取 osu!.db 失败: %v\n", err)
		os.Exit(1)
	}

	collectionHashes, err := db.ReadCollectionDB(collectionDBPath)
	if err != nil {
		fmt.Printf("读取 collection.db 失败: %v\n", err)
		os.Exit(1)
	}

	// 计算缺失的谱面
	missingHashes := utils.SetDifference(collectionHashes, osuHashes)
	if len(missingHashes) == 0 {
		fmt.Println("No missing beatmaps found!")
		return
	}

	fmt.Printf("Found %d missing beatmaps:\n", len(missingHashes))

	// 选择下载类型
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

	fmt.Println("全部下载完成!")
}