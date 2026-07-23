package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/antoni-ostrowski/library-syncer/internal/parser"
	"go.senan.xyz/taglib"
)

const baseApiUrl = "https://api.pillows.su"
const downloadEndpoint = "/api/download/"
const workerCount = 6
const baseCoverPath = "data/covers"

type DebugLogFunc func(format string, a ...any)

const (
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Reset   = "\033[0m"
)

func DownloadTracks(ctx context.Context, sourceTracks *[]parser.Track, outputDir string) {
	colors := []string{Red, Green, Yellow, Blue, Magenta, Cyan}
	tracksCh := make(chan parser.Track)
	var processWg sync.WaitGroup

	devMode, ok := ctx.Value("devMode").(bool)
	if !ok {
		return
	}

	for i := range workerCount {
		processWg.Add(1)

		go func(id int, devMode bool) {
			defer processWg.Done()
			color := colors[id%len(colors)]
			debugLog := func(format string, a ...any) {
				fmt.Printf(color+format+Reset, a...)
			}

			for track := range tracksCh {

				debugLog("[WORKER %v] processing %v \n", id, track.Name)

				for _, link := range track.RealLinks {

					matches, err := filepath.Glob(filepath.Join(outputDir, track.Name+".*"))
					if err == nil && len(matches) > 0 {
						debugLog("[WORKER %v] File %s already exists, skipping...\n", id, track.Name)
						continue
					}

					downloadLink := createDownloadUrl(link)
					if len(downloadLink) == 0 {
						debugLog("[WORKER %v] No download link found", id)
						continue
					}

					debugLog("[WORKER %v] attempting to download %v \n", id, downloadLink)

					finalName, err := downloadFile(downloadLink, track, outputDir, debugLog, fmt.Sprintf("[WORKER %v]", id))
					if err != nil {
						debugLog("Failed to download file %v \n", err)
						continue
					}

					err = taglib.WriteTags(finalName, map[string][]string{
						taglib.Album:     {track.Era},
						taglib.Title:     {track.Name},
						taglib.Artist:    {"yeat"},
						"Notes":          {track.Notes},
						"FileDate":       {track.FileDate},
						"AvailableLen":   {track.AvailableLen},
						"Quality":        {track.Quality},
						"FirstPreview":   {track.FirstPreview},
						"LeakDate":       {track.LeakDate},
						"OGFileLeakDate": {track.OGFileLeakDate},
					}, 0)

					if err != nil {
						debugLog("Failed to write metadata %v \n", err)
						continue
					}

					imageBytes := getImageForTrack(track, baseCoverPath)

					err = taglib.WriteImage(finalName, imageBytes)

					if err != nil {
						debugLog("Failed to embeed image %v \n", err)
						continue
					}

					debugLog("[WORKER %v] successfully downloaded %v \n", id, track.Name)

				}

				if devMode {
					return
				}

			}

		}(i, devMode)

	}

	for _, t := range *sourceTracks {
		tracksCh <- t
	}

	close(tracksCh)

	processWg.Wait()
}

func createDownloadUrl(link string) string {
	var trackId string
	if len(link) >= 32 {
		trackId = link[len(link)-32:]
	} else {
		return ""
	}

	downloadLink := baseApiUrl + downloadEndpoint + trackId
	return downloadLink
}

func downloadFile(downloadLink string, track parser.Track, outputDir string, debugLog DebugLogFunc, workerInfoStr string) (string, error) {
	resp, err := http.Get(downloadLink)
	if err != nil {
		return "", errors.New("Failed to request the download link %v")
	}

	defer resp.Body.Close()

	ext := ".mp3"
	contentType := resp.Header.Get("Content-Type")

	if strings.Contains(contentType, "video/mp4") || strings.Contains(contentType, "audio/mp4") {
		ext = ".mp4"
	} else if strings.Contains(contentType, "audio/x-m4a") || strings.Contains(contentType, "audio/m4a") {
		ext = ".m4a"
	} else if strings.Contains(contentType, "audio/wav") || strings.Contains(contentType, "audio/x-wav") {
		ext = ".wav"
	} else if strings.Contains(contentType, "audio/flac") || strings.Contains(contentType, "audio/x-flac") {
		ext = ".flac"
	} else if strings.Contains(contentType, "audio/mpeg") {
		ext = ".mp3"
	} else if strings.Contains(contentType, "audio/ogg") {
		ext = ".ogg"
	}

	finalName := path.Join(outputDir, track.Name+ext)

	debugLog("%v Saving as: '%v'\n", workerInfoStr, finalName)

	outFile, err := os.Create(finalName)
	if err != nil {
		return "", errors.New("Failed to create out file %v")
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", errors.New("Failed to copy the file from body to out file somehow %v")
	}

	if strings.HasSuffix(finalName, ".mp4") {
		err := processVideoToAudio(finalName, debugLog)
		if err == nil {
			finalName = strings.TrimSuffix(finalName, ".mp4") + ".mp3"
		} else {
			debugLog("Error:", err)
		}
	}

	return finalName, nil

}

func getImageForTrack(track parser.Track, base string) []byte {
	era := strings.TrimSpace(track.Era)
	imagePath := path.Join(base, era+".jpg")

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		imgData, err = os.ReadFile(path.Join(base, "default.jpg"))
		if err != nil {
			return []byte{}
		}
	}

	return imgData
}

func processVideoToAudio(mp4Path string, debugLog DebugLogFunc) error {
	// 1. Create the new filename by replacing .mp4 with .mp3
	mp3Path := strings.TrimSuffix(mp4Path, ".mp4") + ".mp3"

	// 2. Run FFmpeg
	// -i: input
	// -vn: no video
	// -y: overwrite mp3 if it already exists
	cmd := exec.Command("ffmpeg", "-i", mp4Path, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "192k", "-y", mp3Path)

	debugLog("Converting %s to MP3...\n", mp4Path)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("conversion failed: %v", err)
	}

	// 3. Delete the original MP4 file to "replace" it
	err = os.Remove(mp4Path)
	if err != nil {
		return fmt.Errorf("could not delete original mp4: %v", err)
	}

	debugLog("Success! File replaced with MP3.\n")
	return nil
}
