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

	"golang.org/x/sync/semaphore"
)

type Downloader struct {
	songsDir     string
	proxy        string
	workers      int
	delay        time.Duration
	apiToken     string
	downloadType string
	client       *http.Client
}

func NewDownloader(songsDir, proxy string, workers int, delay time.Duration, apiToken, downloadType string) *Downloader {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if proxy != "" {
		client.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
		}
	}

	return &Downloader{
		songsDir:     songsDir,
		proxy:        proxy,
		workers:      workers,
		delay:        delay,
		apiToken:     apiToken,
		downloadType: downloadType,
		client:       client,
	}
}

func (d *Downloader) DownloadAll(hashes map[string]bool) error {
	if err := os.MkdirAll(d.songsDir, 0755); err != nil {
		return fmt.Errorf("Failed to create directory: %v", err)
	}

	sem := semaphore.NewWeighted(int64(d.workers))
	ctx := context.Background()
	var wg sync.WaitGroup
	var lastDownload time.Time

	for hash := range hashes {
		wg.Add(1)
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		go func(h string) {
			defer sem.Release(1)
			defer wg.Done()

			// 控制下载频率
			if elapsed := time.Since(lastDownload); elapsed < d.delay {
				time.Sleep(d.delay - elapsed)
			}

			err := d.downloadBeatmap(h)
			if err != nil {
				fmt.Printf("Download failed: %s, Error: %v\n", h, err)
			} else {
				fmt.Printf("Downloaded: %s\n", h)
			}

			lastDownload = time.Now()
		}(hash)
	}

	wg.Wait()
	return nil
}

func (d *Downloader) downloadBeatmap(md5 string) error {
	// Check if the file already exists
	filePath := filepath.Join(d.songsDir, md5+".osz")
	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	// Generate download links
	links := d.generateDownloadLinks(md5)

	// Try download
	for _, url := range links {
		err := d.tryDownload(url, filePath)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("Every download link failed")
}

func (d *Downloader) generateDownloadLinks(md5 string) []string {
	setID := d.getSetIDFromAPI(md5)
	var links []string

	if setID != 0 {
		switch d.downloadType {
		case "full":
			links = append(links, fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/full/%d", setID))
		case "novideo":
			links = append(links, fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/novideo/%d", setID))
		default: // mini
			links = append(links, fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/mini/%d", setID))
		}
	}

	// 添加基于MD5的下载链接
	links = append(links, fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/novideo/%s", md5))

	return links
}

func (d *Downloader) getSetIDFromAPI(md5 string) int64 {
	if d.apiToken == "" {
		return 0
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://osu.ppy.sh/api/v2/beatmaps/lookup?checksum=%s", md5), nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiToken))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	// 简化的响应解析 - 实际应用中应该使用完整结构
	var result struct {
		Beatmapset struct {
			ID int64 `json:"id"`
		} `json:"beatmapset"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0
	}

	return result.Beatmapset.ID
}

func (d *Downloader) tryDownload(url, filePath string) error {
	resp, err := d.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" && !strings.HasPrefix(contentType, "application/") {
		return fmt.Errorf("Invalid content type: %s", contentType)
	}

	// 创建临时文件
	tmpPath := filePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}

	// 重命名为最终文件
	return os.Rename(tmpPath, filePath)
}
