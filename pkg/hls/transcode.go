package hls

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Job struct {
	MediaID      int
	Input        string
	Output       string // directory for HLS output
	Title        string // metadata title for HLS segments
	Status       string // "transcoding", "done", "error"
	AudioPercent float64
	VideoPercent float64
	Error        string
	cancel       func()
}

var (
	mu   sync.Mutex
	jobs = map[int]*Job{}

	timeRe = regexp.MustCompile(`(\d+):(\d+):(\d+)\.(\d+)`)
)

func Start(mediaID int, input, outDir, title string, onDone func(error)) {
	os.MkdirAll(outDir, 0755)

	job := &Job{
		MediaID: mediaID,
		Input:   input,
		Output:  outDir,
		Title:   title,
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
			job.AudioPercent = 100
			job.VideoPercent = 100
		}
		mu.Unlock()

		if onDone != nil {
			onDone(err)
		}
	}()
}

func Cancel(mediaID int) {
	mu.Lock()
	if j, ok := jobs[mediaID]; ok {
		if j.cancel != nil {
			j.cancel()
		}
		delete(jobs, mediaID)
	}
	mu.Unlock()
}

func GetJob(mediaID int) *Job {
	mu.Lock()
	defer mu.Unlock()
	return jobs[mediaID]
}

func ListJobs() []Job {
	mu.Lock()
	defer mu.Unlock()

	var out []Job
	for _, j := range jobs {
		out = append(out, *j)
	}
	return out
}

func PlaylistPath(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID), "master.m3u8")
}

func HLSDir(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID))
}

// run executes the full transcode pipeline:
// Step 1. Probe input file
// Step 2. Encode audio to AAC (parallel chunks) if needed
// Step 3. Mux video + audio into HLS fMP4
func run(job *Job) error {
	info := probe(job.Input)
	if info.duration <= 0 {
		return fmt.Errorf("cannot probe duration: %s", job.Input)
	}

	needAudioEncode := info.audioCodec != "aac"

	// Step 2. Parallel audio encode
	audioFile := ""
	if needAudioEncode {
		var err error
		audioFile, err = encodeAudio(job, info)
		if err != nil {
			return fmt.Errorf("audio encode: %w", err)
		}
		defer os.Remove(audioFile)
	} else {
		mu.Lock()
		job.AudioPercent = 100 // already AAC, nothing to do
		mu.Unlock()
	}

	// Step 3. Mux into HLS
	needVideoEncode := info.videoCodec != "h264" && info.videoCodec != "hevc"
	return muxHLS(job, info, audioFile, needVideoEncode)
}

// internals

type probeInfo struct {
	duration   float64
	videoCodec string
	audioCodec string
	channels   int
}

func probe(path string) probeInfo {
	var info probeInfo

	out, _ := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	info.duration, _ = strconv.ParseFloat(strings.TrimSpace(string(out)), 64)

	out, _ = exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	info.videoCodec = strings.TrimSpace(string(out))

	out, _ = exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name,channels",
		"-of", "default=noprint_wrappers=1",
		path,
	).Output()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "codec_name=") {
			info.audioCodec = strings.TrimPrefix(line, "codec_name=")
		}
		if strings.HasPrefix(line, "channels=") {
			info.channels, _ = strconv.Atoi(strings.TrimPrefix(line, "channels="))
		}
	}

	return info
}

// encodeAudio extracts audio, splits into chunks, encodes in parallel, merges.
func encodeAudio(job *Job, info probeInfo) (string, error) {
	dir := job.Output
	rawAudio := filepath.Join(dir, "audio_raw.mka")
	defer os.Remove(rawAudio)

	// Step 2a. Extract audio
	if err := exec.Command("ffmpeg", "-v", "quiet",
		"-i", job.Input,
		"-map", "0:a:0", "-vn", "-c:a", "copy",
		"-y", rawAudio,
	).Run(); err != nil {
		return "", fmt.Errorf("extract audio: %w", err)
	}

	// Step 2b. Split into chunks with overlap
	n := runtime.NumCPU()
	if n > 12 {
		n = 12
	}
	overlap := 0.5
	chunkLen := info.duration / float64(n)

	chunks := make([]string, n)
	for i := 0; i < n; i++ {
		ss := float64(i)*chunkLen - overlap
		dur := chunkLen + 2*overlap
		if i == 0 {
			ss = 0
			dur = chunkLen + overlap
		}
		if i == n-1 {
			dur = chunkLen + overlap
		}
		chunks[i] = filepath.Join(dir, fmt.Sprintf("chunk_%d.mka", i))
		if err := exec.Command("ffmpeg", "-v", "quiet",
			"-i", rawAudio,
			"-ss", fmt.Sprintf("%.3f", ss),
			"-t", fmt.Sprintf("%.3f", dur),
			"-c", "copy", "-y", chunks[i],
		).Run(); err != nil {
			return "", fmt.Errorf("split chunk %d: %w", i, err)
		}
	}
	os.Remove(rawAudio)

	// Step 2c. Parallel AAC encode with per-chunk progress
	ac := "2"
	layout := "stereo"
	bitrate := "128k"
	if info.channels >= 6 {
		ac = "6"
		layout = "5.1"
		bitrate = "384k"
	}

	chunkDuration := chunkLen + overlap // each chunk is roughly this long
	progress := make([]float64, n)      // per-chunk progress 0..1
	encoded := make([]string, n)
	var encErr error
	var wg sync.WaitGroup
	var encMu sync.Mutex

	for i := 0; i < n; i++ {
		encoded[i] = filepath.Join(dir, fmt.Sprintf("enc_%d.m4a", i))
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			cmd := exec.Command("ffmpeg", "-v", "quiet",
				"-i", chunks[idx],
				"-c:a", "aac",
				"-ac", ac, "-channel_layout", layout,
				"-b:a", bitrate,
				"-progress", "pipe:1",
				"-y", encoded[idx],
			)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				encMu.Lock()
				if encErr == nil {
					encErr = fmt.Errorf("chunk %d pipe: %w", idx, err)
				}
				encMu.Unlock()
				return
			}
			if err := cmd.Start(); err != nil {
				encMu.Lock()
				if encErr == nil {
					encErr = fmt.Errorf("chunk %d start: %w", idx, err)
				}
				encMu.Unlock()
				return
			}

			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "out_time=") {
					if secs := parseTime(line[9:]); secs > 0 && chunkDuration > 0 {
						p := secs / chunkDuration
						if p > 1 {
							p = 1
						}
						encMu.Lock()
						progress[idx] = p
						// average of all chunks
						var total float64
						for _, v := range progress {
							total += v
						}
						mu.Lock()
						job.AudioPercent = total / float64(n) * 100
						mu.Unlock()
						encMu.Unlock()
					}
				}
			}

			if err := cmd.Wait(); err != nil {
				encMu.Lock()
				if encErr == nil {
					encErr = fmt.Errorf("encode chunk %d: %w", idx, err)
				}
				encMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	for _, f := range chunks {
		os.Remove(f)
	}
	if encErr != nil {
		return "", encErr
	}

	mu.Lock()
	job.AudioPercent = 100
	mu.Unlock()

	// Step 2d. Trim overlap from encoded chunks
	trimmed := make([]string, n)
	for i := 0; i < n; i++ {
		trimmed[i] = filepath.Join(dir, fmt.Sprintf("trim_%d.m4a", i))
		args := []string{"-v", "quiet", "-i", encoded[i]}
		if i > 0 {
			args = append(args, "-ss", fmt.Sprintf("%.3f", overlap))
		}
		args = append(args, "-t", fmt.Sprintf("%.3f", chunkLen))
		args = append(args, "-c", "copy", "-y", trimmed[i])

		if err := exec.Command("ffmpeg", args...).Run(); err != nil {
			return "", fmt.Errorf("trim chunk %d: %w", i, err)
		}
	}

	for _, f := range encoded {
		os.Remove(f)
	}

	// Step 2e. Concat
	listFile := filepath.Join(dir, "concat.txt")
	var lines []string
	for _, f := range trimmed {
		lines = append(lines, fmt.Sprintf("file '%s'", f))
	}
	os.WriteFile(listFile, []byte(strings.Join(lines, "\n")), 0644)

	finalAudio := filepath.Join(dir, "audio.m4a")
	err := exec.Command("ffmpeg", "-v", "quiet",
		"-f", "concat", "-safe", "0",
		"-i", listFile,
		"-c", "copy", "-y", finalAudio,
	).Run()

	for _, f := range trimmed {
		os.Remove(f)
	}
	os.Remove(listFile)

	if err != nil {
		return "", fmt.Errorf("concat audio: %w", err)
	}

	return finalAudio, nil
}

// muxHLS creates the HLS playlist with fMP4 segments.
func muxHLS(job *Job, info probeInfo, audioFile string, needVideoEncode bool) error {
	playlist := filepath.Join(job.Output, "master.m3u8")
	segPattern := filepath.Join(job.Output, "seg_%04d.m4s")

	var args []string

	if needVideoEncode && hasVAAPI() {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	}
	args = append(args, "-i", job.Input)
	if audioFile != "" {
		args = append(args, "-i", audioFile)
	}

	args = append(args, "-map", "0:v:0")
	if audioFile != "" {
		args = append(args, "-map", "1:a:0")
	} else {
		args = append(args, "-map", "0:a:0")
	}

	if needVideoEncode && hasVAAPI() {
		args = append(args, "-vf", "format=nv12,hwupload", "-c:v", "h264_vaapi", "-qp", "20")
	} else if needVideoEncode {
		args = append(args, "-c:v", "libx264", "-preset", "fast", "-crf", "18")
	} else {
		args = append(args, "-c:v", "copy")
	}

	args = append(args, "-c:a", "copy")

	if job.Title != "" {
		args = append(args, "-metadata", "title="+job.Title)
	}

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
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if info.duration > 0 && strings.HasPrefix(line, "out_time=") {
			if secs := parseTime(line[9:]); secs > 0 {
				pct := secs / info.duration * 100
				if pct > 100 {
					pct = 100
				}
				mu.Lock()
				job.VideoPercent = pct
				mu.Unlock()
			}
		}
	}

	return cmd.Wait()
}

func parseTime(s string) float64 {
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
