package hls

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Job struct {
	MediaID  int
	Input    string
	Output   string // directory for HLS output
	Status   string // "transcoding", "done", "error"
	Percent  float64
	Error    string
	cancel   func()
}

var (
	mu   sync.Mutex
	jobs = map[int]*Job{}

	// matches ffmpeg progress: time=00:01:23.45
	timeRe = regexp.MustCompile(`(\d+):(\d+):(\d+)\.(\d+)`)
)

// Start begins transcoding input file to HLS in outDir.
// Calls onDone when finished (with error or nil).
func Start(mediaID int, input, outDir string, onDone func(error)) {
	os.MkdirAll(outDir, 0755)

	job := &Job{
		MediaID: mediaID,
		Input:   input,
		Output:  outDir,
		Status:  "transcoding",
	}

	mu.Lock()
	jobs[mediaID] = job
	mu.Unlock()

	go func() {
		err := run(job)
		mu.Lock()
		if err != nil {
			job.Status = "error"
			job.Error = err.Error()
		} else {
			job.Status = "done"
			job.Percent = 100
		}
		mu.Unlock()

		if onDone != nil {
			onDone(err)
		}
	}()
}

// GetJob returns transcode job status
func GetJob(mediaID int) *Job {
	mu.Lock()
	defer mu.Unlock()
	return jobs[mediaID]
}

// ListJobs returns all transcode jobs
func ListJobs() []Job {
	mu.Lock()
	defer mu.Unlock()

	var out []Job
	for _, j := range jobs {
		out = append(out, *j)
	}
	return out
}

func run(job *Job) error {
	// get total duration first
	duration := probeDuration(job.Input)

	playlist := filepath.Join(job.Output, "master.m3u8")
	segPattern := filepath.Join(job.Output, "seg_%04d.m4s")

	videoCodec := probeVideoCodec(job.Input)
	needVideoTranscode := videoCodec != "h264" && videoCodec != "hevc"
	vaapi := hasVAAPI()

	var args []string

	if needVideoTranscode && vaapi {
		args = append(args,
			"-vaapi_device", "/dev/dri/renderD128",
			"-i", job.Input,
			"-vf", "format=nv12,hwupload",
			"-c:v", "h264_vaapi", "-qp", "20",
		)
	} else if needVideoTranscode {
		args = append(args,
			"-i", job.Input,
			"-c:v", "libx264", "-preset", "fast", "-crf", "18",
		)
	} else {
		args = append(args,
			"-i", job.Input,
			"-c:v", "copy",
		)
	}

	// fMP4 supports all audio codecs natively -- always copy
	args = append(args, "-c:a", "copy")

	args = append(args,
		"-sn",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_segment_filename", segPattern,
		"-progress", "pipe:1",
		"-y",
		playlist,
	)

	cmd := exec.Command("ffmpeg", args...)
	job.cancel = func() { cmd.Process.Kill() }

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = nil // suppress stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	// parse progress from stdout
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if duration > 0 && strings.HasPrefix(line, "out_time=") {
			if secs := parseTime(line[9:]); secs > 0 {
				mu.Lock()
				job.Percent = secs / duration * 100
				if job.Percent > 100 {
					job.Percent = 100
				}
				mu.Unlock()
			}
		}
	}

	return cmd.Wait()
}

func probeDuration(path string) float64 {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0
	}
	d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	return d
}

func probeVideoCodec(path string) string {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func probeAudioCodec(path string) string {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseTime(s string) float64 {
	// format: HH:MM:SS.mmm or HH:MM:SS.mm
	m := timeRe.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	h, _ := strconv.ParseFloat(m[1], 64)
	min, _ := strconv.ParseFloat(m[2], 64)
	sec, _ := strconv.ParseFloat(m[3], 64)
	frac, _ := strconv.ParseFloat("0."+m[4], 64)
	return h*3600 + min*60 + sec + frac
}

func hasVAAPI() bool {
	_, err := os.Stat("/dev/dri/renderD128")
	return err == nil
}

// PlaylistPath returns path to master.m3u8 for a media
func PlaylistPath(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID), "master.m3u8")
}

// HLSDir returns HLS output directory for a media
func HLSDir(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID))
}
