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
	"strconv"
	"strings"

	"go.senan.xyz/taglib"
)

const (
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Reset   = "\033[0m"
)

var baseCoverPath = os.Getenv("ASSETS_PATH")

type DebugLogFunc func(format string, a ...any)

type Track struct {
	Artist string
	Era    string
	Name   string
	Notes  string
	Links  string
}

type Source int

const (
	SourcePillowcase = iota
	SourceSc
)

type Downloadable interface {
	Download(int) error
}

type DownloadableTrack struct {
	Track  Track
	Url    string
	Source Source
}

func (d *DownloadableTrack) Download(workerId int) error {
	switch d.Source {
	case SourcePillowcase:
		return d.downloadPillowcase(workerId)
	case SourceSc:
		return d.downloadSc(workerId)
	default:
		return fmt.Errorf("unkown source")
	}
}

func (d *DownloadableTrack) downloadPillowcase(workerId int) error {
	link := d.Url
	t := d.Track

	var outputDir = os.Getenv("SONGS_PATH")
	colors := []string{Red, Green, Yellow, Blue, Magenta, Cyan}

	color := colors[workerId%len(colors)]
	debugLog := func(format string, a ...any) {
		fmt.Printf(color+"[WORKER %v] "+format+Reset, append([]any{workerId}, a...)...)
	}

	debugLog("processing %v \n", t.Name)
	tId := getTrackId(link)
	matches, err := filepath.Glob(filepath.Join(outputDir, t.Name+tId+".*"))
	if err == nil && len(matches) > 0 {
		debugLog("File %s already exists, skipping...\n", t.Name)
		return err
	}

	if len(link) == 0 {
		debugLog("No download link found\n")
		return nil
	}

	debugLog("attempting to download %v \n", link)

	finalName, err := downloadFile(link, t, outputDir, debugLog)
	if err != nil {
		debugLog("Failed to download file %v \n", err)
		return err
	}
	finalName, err = Amplify(finalName)
	if err != nil {
		return err
	}

	err = writeMetadata(finalName, t)
	if err != nil {
		debugLog("Failed to write metadata %v \n", err)
		return err
	}

	debugLog("successfully downloaded %v \n", t.Name)

	return nil

}

func (d *DownloadableTrack) downloadSc(workerId int) error {
	t := d.Track
	link := d.Url

	var outputDir = os.Getenv("SONGS_PATH")
	colors := []string{Red, Green, Yellow, Blue, Magenta, Cyan}

	color := colors[workerId%len(colors)]
	debugLog := func(format string, a ...any) {
		fmt.Printf(color+"[WORKER %v] "+format+Reset, append([]any{workerId}, a...)...)
	}

	debugLog("processing %v \n", t.Name)
	tId := getTrackSlug(link)
	matches, err := filepath.Glob(filepath.Join(outputDir, t.Name+tId+".*"))
	if err == nil && len(matches) > 0 {
		debugLog("File %s already exists, skipping...\n", t.Name)
		return nil
	}

	if len(link) == 0 {
		debugLog("No download link found\n")
		return nil
	}

	debugLog("attempting to download %v \n", link)

	outputTemplate := filepath.Join(outputDir, t.Name+tId+".%(ext)s")
	cmd := exec.Command(
		"yt-dlp",
		"-f", "hls_aac_160k/http_mp3_1_0/bestaudio",
		"-o", outputTemplate,
		link,
	)

	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		fmt.Println("yt-dlp failed:", err)
	}

	matches, err = filepath.Glob(filepath.Join(outputDir, t.Name+tId+".*"))
	if err != nil {
		fmt.Printf("failed to find the downloaded file?%v \n", err)
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no downloaded file found for %s", t.Name)
	}
	finalName := matches[0]

	finalName, err = Amplify(finalName)
	if err != nil {
		return err
	}

	err = writeMetadata(finalName, t)
	if err != nil {
		debugLog("Failed to write metadata %v \n", err)
		return err
	}

	debugLog("successfully downloaded %v \n", t.Name)

	return nil

}

func StartWorkers(ctx context.Context, devMode bool, tracksToDownload <-chan Downloadable) {
	workerCount := GetWorkerCount()
	for id := range workerCount {
		go func(id int) {
			for track := range tracksToDownload {
				track.Download(id)

				if devMode {
					return
				}

			}

		}(id)

	}

}

func downloadFile(link string, track Track, outputDir string, debugLog DebugLogFunc) (string, error) {
	resp, err := http.Get(link)
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

	trackId := getTrackId(link)
	finalName := path.Join(outputDir, track.Name+trackId+ext)

	debugLog("Saving as: '%v'\n", finalName)

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
			debugLog("Error: %v\n", err)
		}
	}

	return finalName, nil

}

func getImageForTrack(track Track, base string) []byte {
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
func GetWorkerCount() int {
	s := os.Getenv("WORKER_COUNT")
	if s == "" {
		return 4
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 4
	}
	return n
}

func getTrackId(link string) string {
	s := link
	trackId := ""
	if len(s) >= 32 {
		trackId = s[len(s)-32:]
	}
	return "---" + trackId
}

func getTrackSlug(link string) string {
	const prefix = "soundcloud.com/"
	idx := strings.Index(link, prefix)
	if idx == -1 {
		return ""
	}

	slug := link[idx+len(prefix):]
	if q := strings.IndexAny(slug, "?#"); q != -1 {
		slug = slug[:q]
	}
	slug = strings.TrimSuffix(slug, "/")
	slug = strings.ReplaceAll(slug, "/", "-")

	return "---" + slug
}

func writeMetadata(filepath string, t Track) error {
	existing, _ := taglib.ReadTags(filepath)

	tags := map[string][]string{
		taglib.Album:           {t.Era},
		taglib.Title:           {t.Name},
		taglib.Comment:         {t.Notes},
		taglib.Artist:          {t.Artist},
		taglib.AlbumArtist:     {t.Artist},
		taglib.ArtistSort:      {t.Artist},
		taglib.AlbumArtistSort: {t.Artist},
		taglib.Composer:        {t.Artist},
		taglib.Artists:         {t.Artist},
		taglib.Conductor:       {t.Artist},
		taglib.Performer:       {t.Artist},
		taglib.Remixer:         {t.Artist},
		taglib.OriginalArtist:  {t.Artist},
	}

	if dates, ok := existing[taglib.Date]; ok {
		tags[taglib.Date] = dates
	}

	if releaseDate, ok := existing[taglib.ReleaseDate]; ok {
		tags[taglib.ReleaseDate] = releaseDate
	}

	if ogDate, ok := existing[taglib.OriginalDate]; ok {
		tags[taglib.OriginalDate] = ogDate
	}

	if err := taglib.WriteTags(filepath, tags, taglib.Clear); err != nil {
		return fmt.Errorf("write tags: %w", err)
	}

	imageBytes := getImageForTrack(t, baseCoverPath)
	if len(imageBytes) > 0 {
		if err := taglib.WriteImage(filepath, imageBytes); err != nil {
			return fmt.Errorf("write image: %w", err)
		}
	}

	return nil
}

// AmplifyMP3 makes the file 7 dB louder with a limiter and replaces the
// original via a temp file (so the change looks "in-place").
func Amplify(inputPath string) (string, error) {
	dir := filepath.Dir(inputPath)
	outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".mp3"

	tmp, err := os.CreateTemp(dir, "*.mp3")
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}

	tmpPath := tmp.Name()
	tmp.Close()

	defer func() {
		if _, err := os.Stat(tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		}
	}()

	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath,
		"-map", "0:a:0",
		"-map", "0:v?",
		"-map_metadata", "0",
		"-map_chapters", "0",
		"-filter:a", "volume=7.0dB,alimiter=limit=0.95",
		"-c:a", "libmp3lame",
		"-b:a", "320k",
		"-c:v", "copy",
		"-disposition:v", "attached_pic",
		"-id3v2_version", "3",
		"-y", tmpPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg failed: %w\n%s", err, out)
	}

	if err := os.Rename(tmpPath, outputPath); err != nil {
		return "", fmt.Errorf("rename output: %w", err)
	}

	if inputPath != outputPath {
		_ = os.Remove(inputPath)
	}

	return outputPath, nil
}
