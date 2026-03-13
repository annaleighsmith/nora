package utils

import (
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/annaleighsmith/nora/config"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// StripCodeFences removes markdown code fences wrapping AI-formatted content.
func StripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if i := strings.Index(s, "\n"); i != -1 {
			s = s[i+1:]
		}
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimRight(s, "\n")
	}
	return s
}

// Slugify converts a string to a URL-safe slug in the given style (kebab/snake/pascal).
func Slugify(s, style string) string {
	switch style {
	case "snake":
		return slugifyWith(s, "_")
	case "pascal":
		return pascalize(s)
	default: // "kebab"
		return slugifyWith(s, "-")
	}
}

func slugifyWith(s string, sep string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, sep)
	s = strings.Trim(s, sep)
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, sep)
	}
	return s
}

func pascalize(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	words := reg.Split(s, -1)
	var result strings.Builder
	for _, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}
	out := result.String()
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

// GenerateFilename creates a filename from formatted note content
// using the configured naming convention.
func GenerateFilename(content string) string {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	fm, _ := ParseFrontmatter(content)
	title := fm.Title
	if title == "" {
		title = "note"
	}
	date := fm.Date
	if date == "" {
		date = time.Now().Format(cfg.Format.DateFormat)
	}
	slug := Slugify(title, cfg.Format.SlugStyle)

	name := cfg.Format.Naming
	name = strings.ReplaceAll(name, "{date}", date)
	name = strings.ReplaceAll(name, "{slug}", slug)
	return name + ".md"
}

// GenerateFilenameFromSlug uses a custom slug instead of extracting the title,
// but still applies the configured naming convention (date, slug style).
func GenerateFilenameFromSlug(content, customName string) string {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	slug := strings.TrimSuffix(customName, ".md")
	slug = Slugify(slug, cfg.Format.SlugStyle)

	fm, _ := ParseFrontmatter(content)
	date := fm.Date
	if date == "" {
		date = time.Now().Format(cfg.Format.DateFormat)
	}

	name := cfg.Format.Naming
	name = strings.ReplaceAll(name, "{date}", date)
	name = strings.ReplaceAll(name, "{slug}", slug)
	return name + ".md"
}
