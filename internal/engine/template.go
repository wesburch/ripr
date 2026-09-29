package engine

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxNameBytes = 180

func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		if unicode.IsControl(r) {
			return '-'
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

func capBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func expand(template string, meta Meta, format Format) string {
	if strings.TrimSpace(template) == "" {
		template = "{artist} - {title}.{ext}"
	}
	artist, title := meta.Artist, meta.Title
	if artist == "" {
		artist = "Unknown Artist"
	}
	if title == "" {
		title = "audio"
	}
	hasExt := strings.Contains(template, "{ext}")
	out := strings.NewReplacer(
		"{artist}", artist,
		"{title}", title,
		"{uploader}", meta.Uploader,
		"{year}", meta.Year,
		"{ext}", string(format),
		"{id}", meta.ID,
		"{source}", meta.Extractor,
	).Replace(template)
	out = sanitize(out)
	out = strings.TrimLeft(out, ".")
	if hasExt {
		suffix := "." + string(format)
		base := strings.TrimSuffix(out, suffix)
		base = strings.TrimRight(capBytes(base, maxNameBytes-len(suffix)), " .")
		if base == "" {
			base = "audio"
		}
		return base + suffix
	}
	out = strings.TrimSpace(capBytes(out, maxNameBytes))
	if out == "" {
		out = "audio"
	}
	return out
}
