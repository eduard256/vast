package hls

import (
	"bufio"
	"encoding/json"
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
	MediaID int
	Input   string
	Output  string
	Status  string
	Percent float64
	Error   string
	cancel  func()
}

type AudioTrack struct {
	Index    int    `json:"index"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels"`
}

var (
	mu   sync.Mutex
	jobs = map[int]*Job{}

	timeRe = regexp.MustCompile(`(\d+):(\d+):(\d+)\.(\d+)`)
)

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

func GetJob(mediaID int) *Job {
	mu.Lock()
	defer mu.Unlock()
	return jobs[mediaID]
}

func Cancel(mediaID int) {
	mu.Lock()
	if job, ok := jobs[mediaID]; ok {
		if job.cancel != nil {
			job.cancel()
		}
		delete(jobs, mediaID)
	}
	mu.Unlock()
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

func run(job *Job) error {
	duration := probeDuration(job.Input)
	tracks := ProbeAudioTracks(job.Input)
	videoCodec := probeVideoCodec(job.Input)
	needVideoTranscode := videoCodec != "h264" && videoCodec != "hevc"
	vaapi := hasVAAPI()

	if len(tracks) == 0 {
		tracks = []AudioTrack{{Index: 0, Language: "und", Title: "Audio", Codec: "unknown"}}
	}

	// build ffmpeg command with separate outputs for video + each audio track
	var args []string

	// input and hw accel
	if needVideoTranscode && vaapi {
		args = append(args, "-vaapi_device", "/dev/dri/renderD128")
	}
	args = append(args, "-i", job.Input)

	// video output
	videoPlaylist := filepath.Join(job.Output, "video.m3u8")
	videoSegPattern := filepath.Join(job.Output, "video_%04d.m4s")

	if needVideoTranscode && vaapi {
		args = append(args,
			"-map", "0:v:0",
			"-c:v", "h264_vaapi", "-qp", "20",
			"-vf", "format=nv12,hwupload",
		)
	} else if needVideoTranscode {
		args = append(args,
			"-map", "0:v:0",
			"-c:v", "libx264", "-preset", "fast", "-crf", "18",
		)
	} else {
		args = append(args,
			"-map", "0:v:0",
			"-c:v", "copy",
		)
	}
	args = append(args,
		"-an", "-sn",
		"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "video_init.mp4",
		"-hls_segment_filename", videoSegPattern,
		"-y", videoPlaylist,
	)

	// audio outputs -- one per track
	// codecs that browsers can decode in fMP4 HLS natively
	browserCodecs := map[string]bool{
		"aac": true, "ac3": true, "eac3": true,
		"mp3": true, "opus": true, "vorbis": true,
	}

	for i, t := range tracks {
		audioPlaylist := filepath.Join(job.Output, fmt.Sprintf("audio_%d.m3u8", i))
		audioSegPattern := filepath.Join(job.Output, fmt.Sprintf("audio_%d_%%04d.m4s", i))

		// copy if browser-compatible, otherwise transcode to AAC preserving channels
		var audioCodecArgs []string
		if browserCodecs[t.Codec] {
			audioCodecArgs = []string{"-c:a", "copy"}
		} else {
			// DTS, DTS-HD, TrueHD, FLAC, PCM -> AAC
			audioCodecArgs = []string{"-c:a", "aac", "-b:a", "640k"}
		}

		args = append(args,
			"-map", fmt.Sprintf("0:a:%d", t.Index),
		)
		args = append(args, audioCodecArgs...)
		args = append(args,
			"-vn", "-sn",
			"-movflags", "+frag_keyframe+empty_moov+default_base_moof",
			"-hls_time", "6",
			"-hls_playlist_type", "vod",
			"-hls_segment_type", "fmp4",
			"-hls_fmp4_init_filename", fmt.Sprintf("audio_%d_init.mp4", i),
			"-hls_segment_filename", audioSegPattern,
			"-y", audioPlaylist,
		)
	}

	args = append(args, "-progress", "pipe:1")

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

	if err := cmd.Wait(); err != nil {
		return err
	}

	// generate master.m3u8 with EXT-X-MEDIA for audio tracks
	return writeMasterPlaylist(job.Output, tracks)
}

func writeMasterPlaylist(outDir string, tracks []AudioTrack) error {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")

	// find default audio track: first russian, otherwise first track
	defaultIdx := 0
	for i, t := range tracks {
		if t.Language == "rus" || t.Language == "ru" {
			defaultIdx = i
			break
		}
	}

	// write EXT-X-MEDIA entries for each audio track
	for i, t := range tracks {
		name := t.Title
		if name == "" {
			name = t.Language
		}
		if name == "" {
			name = fmt.Sprintf("Track %d", i+1)
		}

		lang := t.Language
		if lang == "" {
			lang = "und"
		}

		isDefault := "NO"
		if i == defaultIdx {
			isDefault = "YES"
		}

		b.WriteString(fmt.Sprintf(
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",LANGUAGE=\"%s\",DEFAULT=%s,AUTOSELECT=%s,URI=\"audio_%d.m3u8\"\n",
			name, lang, isDefault, isDefault, i,
		))
	}

	b.WriteString("\n")

	// read video playlist to get bandwidth estimate
	b.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=5000000,AUDIO=\"audio\"\n")
	b.WriteString("video.m3u8\n")

	return os.WriteFile(filepath.Join(outDir, "master.m3u8"), []byte(b.String()), 0644)
}

// ProbeAudioTracks returns all audio tracks in a media file
func ProbeAudioTracks(path string) []AudioTrack {
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,channels:stream_tags=language,title",
		"-of", "json",
		path,
	).Output()
	if err != nil {
		return nil
	}

	var result struct {
		Streams []struct {
			Index    int `json:"index"`
			Codec    string `json:"codec_name"`
			Channels int    `json:"channels"`
			Tags     struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	var tracks []AudioTrack
	for i, s := range result.Streams {
		tracks = append(tracks, AudioTrack{
			Index:    i, // audio stream index (not absolute)
			Language: s.Tags.Language,
			Title:    s.Tags.Title,
			Codec:    s.Codec,
			Channels: s.Channels,
		})
	}
	return tracks
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

func PlaylistPath(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID), "master.m3u8")
}

func HLSDir(dataDir string, mediaID int) string {
	return filepath.Join(dataDir, "hls", fmt.Sprintf("%d", mediaID))
}
