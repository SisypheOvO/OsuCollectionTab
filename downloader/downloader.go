package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
		Timeout: 120 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
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

func (d *Downloader) SetDownloadType(downloadType string) {
    d.downloadType = downloadType
}

func (d *Downloader) DownloadAll(setIDs map[int64]struct{}) error {
	if err := os.MkdirAll(d.songsDir, 0755); err != nil {
		return fmt.Errorf("Failed to create directory: %v", err)
	}

	sem := semaphore.NewWeighted(int64(d.workers))
	ctx := context.Background()
	var wg sync.WaitGroup
	var lastDownload time.Time

	for setID := range setIDs {
		wg.Add(1)
		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		go func(id int64) {
			defer sem.Release(1)
			defer wg.Done()

			// 控制下载速率
			if elapsed := time.Since(lastDownload); elapsed < d.delay {
				time.Sleep(d.delay - elapsed)
			}

			err := d.downloadBeatmapSet(id)
			if err != nil {
				fmt.Printf("Download failed for set %d: %v\n", id, err)
			} else {
				fmt.Printf("Downloaded set %d\n", id)
			}

			lastDownload = time.Now()
		}(setID)
	}

	wg.Wait()
	return nil
}

func (d *Downloader) downloadBeatmapSet(setID int64) error {
	fmt.Printf("Downloading set %d\n", setID)
	url := fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/%s/%d", d.downloadType, setID)
	finalPath := filepath.Join(d.songsDir, fmt.Sprintf("%d.osz", setID))
	return d.tryDownload(url, finalPath)
}

func (d *Downloader) GetSetIDFromAPI(md5 string) int64 {
	if d.apiToken == "" {
		return 0
	}

	// v2 API 需要 Bearer Token
	// req, err := http.NewRequest("GET", fmt.Sprintf("https://osu.ppy.sh/api/v2/beatmaps/lookup?checksum=%s", md5), nil)
	// if err != nil {
	// 	return 0
	// }
	// req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiToken))

	// v1 只需要固定的 Token
	req, err := http.NewRequest("GET", fmt.Sprintf("https://osu.ppy.sh/api/get_beatmaps?k=%s&h=%s", d.apiToken, md5), nil)
	if err != nil {
		return 0
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0
	}

	var response []struct {
		SetID string `json:"beatmapset_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		fmt.Printf("Failed to decode JSON: %v\n", err)
		return 0
	}

	if len(response) == 0 {
		return 0
	}

	setID, err := strconv.ParseInt(response[0].SetID, 10, 64)
	if err != nil {
		return 0
	}

	return setID
}

func (d *Downloader) tryDownload(targetUrl, filePath string) error {
	fmt.Printf("Downloading from %s\n", targetUrl)
	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "keep-alive")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" && !strings.HasPrefix(contentType, "application/") {
		return fmt.Errorf("Invalid content type: %s", contentType)
	}

	// Extract filename from Content-Disposition header
	contentDisposition := resp.Header.Get("Content-Disposition")
	var filename string
	if contentDisposition != "" {
		// Try to extract filename from filename="..." pattern
		if start := strings.Index(contentDisposition, "filename=\""); start != -1 {
			start += len("filename=\"")
			end := strings.Index(contentDisposition[start:], "\"")
			if end != -1 {
				filename = contentDisposition[start : start+end]
				// Handle URL encoded filenames
				if decoded, err := url.QueryUnescape(filename); err == nil {
					filename = decoded
				}
			}
		}
		// Fallback to filename* (RFC 5987)
		if filename == "" {
			if start := strings.Index(contentDisposition, "filename*="); start != -1 {
				value := contentDisposition[start+len("filename*="):]
				// Handle UTF-8 encoded filenames (format: utf-8''filename)
				if strings.HasPrefix(value, "utf-8''") {
					filename = value[len("utf-8''"):]
					// Remove any trailing parameters or quotes
					if end := strings.IndexAny(filename, "\";"); end != -1 {
						filename = filename[:end]
					}
					if decoded, err := url.QueryUnescape(filename); err == nil {
						filename = decoded
					}
				}
			}
		}
	}

	// Start to write to a temp file
	tmpPath := filePath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		fmt.Printf("Failed to create temp file: %v\n", err)
		return err
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Failed to write to temp file: %v\n", err)
		out.Close()
		os.Remove(tmpPath)
		return err
	}

	out.Close()

	// Determine the final file path
	finalPath := filePath
	if filename != "" {
		// Use the extracted filename (but keep the original directory)
		finalPath = filepath.Join(filepath.Dir(filePath), filename)
	}

	// Retry renaming the temp file to the final name
	for attempts := 0; attempts < 3; attempts++ {
		err := os.Rename(tmpPath, finalPath)
		if err == nil {
			return nil
		}

		if attempts < 2 { // before the last attempt
			fmt.Printf("Rename attempt %d failed: %v. Retrying after delay...\n", attempts+1, err)
			time.Sleep(500 * time.Millisecond) // Wait before retrying
		} else {
			return fmt.Errorf("failed to rename file after multiple attempts: %v", err)
		}
	}

	return nil
}
