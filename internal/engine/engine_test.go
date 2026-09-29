package engine

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExpand(t *testing.T) {
	m := Meta{ID: "abc", Title: "Song: A/B?", Artist: "Ar*tist", Uploader: "up", Year: "2004", Extractor: "youtube"}
	got := Expand("{artist} – {title}.{ext}", m, MP3)
	if got != "Ar-tist – Song- A-B-.mp3" {
		t.Fatalf("got %q", got)
	}
	if got := Expand("{id} {source} {year} {uploader}", m, MP3); got != "abc youtube 2004 up" {
		t.Fatalf("got %q", got)
	}
	if got := Expand("{artist} - {title}", Meta{}, FLAC); got != "Unknown Artist - audio" {
		t.Fatalf("got %q", got)
	}
	// ext guarantee
	if got := Expand("{title}.{ext}", Meta{Title: "x"}, FLAC); got != "x.flac" {
		t.Fatalf("got %q", got)
	}
	if got := Expand("{title}.{ext}", Meta{Title: "x"}, WAV); got != "x.wav" {
		t.Fatalf("got %q", got)
	}
	// whitespace + control + separators
	got = Expand("a\t\n  b \\ /c.{ext}", m, M4A)
	if strings.ContainsAny(got, "/\\\n\t") || strings.Contains(got, "  ") {
		t.Fatalf("got %q", got)
	}
	// length cap without splitting runes, ext survives
	long := Meta{Title: strings.Repeat("é", 300), Artist: "a"}
	got = Expand("{artist} {title}.{ext}", long, MP3)
	if len(got) > 180 || !utf8.ValidString(got) || !strings.HasSuffix(got, ".mp3") {
		t.Fatalf("len %d %q", len(got), got)
	}
	got = Expand("{title}", long, MP3)
	if len(got) > 180 || !utf8.ValidString(got) {
		t.Fatalf("len %d", len(got))
	}
	if got := Expand("../../{title}", Meta{Title: "x"}, MP3); strings.ContainsAny(got, "/\\") || strings.HasPrefix(got, ".") {
		t.Fatalf("got %q", got)
	}
}

func TestParseRiprLine(t *testing.T) {
	ev, ok := parseRiprLine("RIPR 500 1000 NA 2048.5")
	if !ok || ev.Bytes != 500 || ev.TotalBytes != 1000 || ev.Speed != 2048.5 || ev.Progress != 0.5 || ev.Phase != PhasePull || ev.Column != -1 {
		t.Fatalf("%+v", ev)
	}
	ev, ok = parseRiprLine("RIPR 250 NA 1000 NA")
	if !ok || ev.TotalBytes != 1000 || ev.Speed != 0 || ev.Progress != 0.25 {
		t.Fatalf("%+v", ev)
	}
	ev, ok = parseRiprLine("RIPR NA NA NA NA")
	if !ok || ev.Bytes != 0 || ev.TotalBytes != 0 || ev.Progress != 0 {
		t.Fatalf("%+v", ev)
	}
	if _, ok := parseRiprLine("[download] Destination: x"); ok {
		t.Fatal("should not parse")
	}
}

func TestParseKVAndProgress(t *testing.T) {
	var st progressState
	lines := []string{"frame=0", "out_time_us=1500000", "out_time_ms=1500000", "speed=12.3x", "progress=continue"}
	done := false
	for _, l := range lines {
		k, v, ok := parseKV(l)
		if !ok {
			t.Fatal(l)
		}
		done = st.feed(k, v)
	}
	if !done || st.Time != 1.5 || st.SpeedX != 12.3 || st.End {
		t.Fatalf("%+v", st)
	}
	k, v, _ := parseKV("speed=N/A")
	st.feed(k, v)
	if st.SpeedX != 0 {
		t.Fatal("N/A speed")
	}
	k, v, _ = parseKV("progress=end")
	st.feed(k, v)
	if !st.End {
		t.Fatal("end")
	}
	if _, _, ok := parseKV("Error opening input: No such file"); ok {
		t.Fatal("error line parsed as kv")
	}
	if _, _, ok := parseKV("[mp3 @ 0x1] bad value = 3"); ok {
		t.Fatal("error line parsed as kv")
	}
}

func TestParseFfmpegVersion(t *testing.T) {
	if v := parseFfmpegVersion("ffmpeg version 9.0.2 Copyright (c) 2000\nbuilt with"); v != "9.0.2" {
		t.Fatal(v)
	}
}

func TestParseProbe(t *testing.T) {
	m, err := parseProbe([]byte(`{"id":"x","title":"T","track":"","uploader":"U","channel":"C","upload_date":"20050423",
	"duration":19,"extractor_key":"YoutubeTab","acodec":"opus","abr":130.5,"asr":48000,"audio_channels":2,
	"chapters":[{"title":"a","start_time":0,"end_time":5}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "T" || m.Artist != "U" || m.Year != "2005" || m.Extractor != "youtube" || m.SampleRate != 48000 || len(m.Chapters) != 1 {
		t.Fatalf("%+v", m)
	}
	if normalizeExtractor("Youtube:tab") != "youtube" || normalizeExtractor("SoundcloudUser") != "soundcloud" {
		t.Fatal("extractor")
	}
}

func TestEstimate(t *testing.T) {
	m := Meta{Duration: 100}
	if got := Estimate(m, MP3, 192); got != 2_400_000 {
		t.Fatal(got)
	}
	if got := Estimate(m, FLAC, 0); got != 11_250_000 {
		t.Fatal(got)
	}
	if got := Estimate(m, WAV, 0); got != 17_637_500 {
		t.Fatal(got)
	}
	if Estimate(Meta{}, MP3, 192) != 0 {
		t.Fatal("zero duration")
	}
}

func TestFfmpegArgs(t *testing.T) {
	job := Job{Format: MP3, Bitrate: 192, CoverArt: true, Tags: true, Normalize: true, Trim: &[2]float64{1, 3},
		Meta: Meta{Title: "T"}}
	a := ffmpegArgs(job, "src.webm", "c.jpg", "out.mp3")
	s := strings.Join(a, " ")
	for _, want := range []string{"-ss 1 -to 3 -i src.webm -i c.jpg -map 0:a -map 1:v", "-af loudnorm", "-metadata title=T", "out.mp3 -map 0:a -ac 2 -ar 4000 -f s16le pipe:1"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Count(s, "-map 1:v") != 1 || strings.Count(s, "loudnorm") != 1 {
		t.Error("cover/loudnorm leaked to tap")
	}
	if !slices.Contains(ffmpegArgs(Job{Format: WAV}, "s", "c", "o"), "pcm_s16le") {
		t.Error("wav codec")
	}
}

func sinePCM(seconds float64) []byte {
	n := int(seconds * tapRate)
	var b bytes.Buffer
	for i := 0; i < n; i++ {
		v := int16(math.Sin(2*math.Pi*440*float64(i)/tapRate) * 30000)
		binary.Write(&b, binary.LittleEndian, v)
		binary.Write(&b, binary.LittleEndian, v)
	}
	return b.Bytes()
}

func TestFold(t *testing.T) {
	pcm := sinePCM(2)
	var evs []Event
	amps, err := fold(bytes.NewReader(pcm), 2, 192, func(e Event) bool { evs = append(evs, e); return true })
	if err != nil {
		t.Fatal(err)
	}
	if len(amps) != 192 {
		t.Fatal(len(amps))
	}
	for i, a := range amps {
		if a <= 0.5 {
			t.Fatalf("amp[%d]=%v", i, a)
		}
	}
	levels, cols := 0, 0
	for _, e := range evs {
		if e.Phase != PhaseDecode {
			t.Fatal("phase")
		}
		if e.Column == -1 {
			levels++
			if e.LevelL <= 0.5 || e.LevelR <= 0.5 || e.LevelL > 1 {
				t.Fatalf("level %+v", e)
			}
		} else {
			if e.Column != cols {
				t.Fatalf("column order: got %d want %d", e.Column, cols)
			}
			cols++
		}
	}
	if levels < 38 || levels > 42 || cols != 192 {
		t.Fatalf("levels %d cols %d", levels, cols)
	}
	sp := spark(amps)
	for _, v := range sp {
		if v > 3 || v == 0 {
			t.Fatalf("spark %v", sp)
		}
	}
}

func TestFoldOddChunks(t *testing.T) {
	pcm := sinePCM(1)
	want, _ := fold(bytes.NewReader(pcm), 1, 64, func(Event) bool { return true })
	got, err := fold(&chunkReader{b: pcm}, 1, 64, func(Event) bool { return true })
	if err != nil || !slices.Equal(want, got) {
		t.Fatalf("mismatch err=%v", err)
	}
}

type chunkReader struct{ b []byte }

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.b) == 0 {
		return 0, io.EOF
	}
	n := min(7, len(p), len(c.b))
	copy(p, c.b[:n])
	c.b = c.b[n:]
	return n, nil
}

func TestFoldAbort(t *testing.T) {
	_, err := fold(bytes.NewReader(sinePCM(1)), 1, 16, func(Event) bool { return false })
	if err == nil {
		t.Fatal("expected abort error")
	}
}

func TestSparkQuantize(t *testing.T) {
	amps := make([]float32, 32)
	amps[0], amps[2], amps[4], amps[6] = 0.0, 0.2, 0.5, 1.0
	sp := spark(amps)
	if sp[1] != 1 || sp[2] != 2 || sp[3] != 3 {
		t.Fatalf("spark %v", sp)
	}
}
