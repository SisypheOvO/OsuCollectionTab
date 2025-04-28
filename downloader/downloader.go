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

			// Control download rate
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

	fmt.Printf("Trying to download %s from %s\n", md5, strings.Join(links, ", "))

	// Try download
	for _, url := range links {
		err := d.tryDownload(url, filePath)
		if err != nil {
			fmt.Printf("Failed to download from %s: %v\n", url, err)
			continue
		}
		fmt.Printf("Downloaded from %s\n", url)
		return nil
	}

	return fmt.Errorf("Every download link failed")
}

func (d *Downloader) generateDownloadLinks(md5 string) []string {
	setID := d.getSetIDFromAPI(md5)
	var links []string

	// Actually sayo has a download API to download beatmaps by md5
	// eg: https://dl.sayobot.cn/beatmaps/download/osz/25a63e7d375da46e74c12b0455de7be4
	// not used here kinda lazy to implement

	if setID != 0 {
		links = append(links, fmt.Sprintf("https://dl.sayobot.cn/beatmaps/download/%s/%d", d.downloadType, setID))
	}

	return links
}

func (d *Downloader) getSetIDFromAPI(md5 string) int64 {
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

	// body 示例如下
	// [
	//     {
	//         "beatmapset_id": "796338",
	//         "beatmap_id": "1672217",
	//         "approved": "1",
	//         "total_length": "80",
	//         "hit_length": "80",
	//         "version": "Insane",
	//         "file_md5": "67a672ab9d4bf2b12e8155b595740883",
	//         "diff_size": "4",
	//         "diff_overall": "7.8",
	//         "diff_approach": "9",
	//         "diff_drain": "5.8",
	//         "mode": "0",
	//         "count_normal": "109",
	//         "count_slider": "128",
	//         "count_spinner": "0",
	//         "submit_date": "2018-06-11 01:45:06",
	//         "approved_date": "2018-07-13 08:00:12",
	//         "last_update": "2018-07-04 05:13:55",
	//         "artist": "Trial & Error",
	//         "artist_unicode": "Trial & Error",
	//         "title": "Taiatari*Romance feat. Ayuru Ouhashi (Short Ver.)",
	//         "title_unicode": "たいあたり★ロマンス feat. Ayuru Ouhashi (Short Ver.)",
	//         "creator": "Affirmation",
	//         "creator_id": "6186628",
	//         "bpm": "170",
	//         "source": "",
	//         "tags": "featured artist [_kuro_usagi_] and",
	//         "genre_id": "2",
	//         "language_id": "3",
	//         "favourite_count": "216",
	//         "rating": "9.1658",
	//         "storyboard": "1",
	//         "video": "0",
	//         "download_unavailable": "0",
	//         "audio_unavailable": "0",
	//         "playcount": "228492",
	//         "passcount": "75114",
	//         "packs": "S672,T100",
	//         "max_combo": "384",
	//         "diff_aim": "2.25792",
	//         "diff_speed": "1.70401",
	//         "difficultyrating": "4.24434"
	//     }
	// ]

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
