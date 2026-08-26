package sources

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	packerRegex = regexp.MustCompile(`}\('(.*)',\s*(\d+),\s*(\d+),\s*'([^']*)'\.split\('\|'\)`)
)

// UnpackDeanEdwards unpacks a JavaScript string packed with Dean Edwards Packer
func UnpackDeanEdwards(packed string) string {
	matches := packerRegex.FindStringSubmatch(packed)
	if len(matches) < 5 {
		return packed
	}

	payload := matches[1]
	radixStr := matches[2]
	countStr := matches[3]
	symtabStr := matches[4]

	radix, err := strconv.Atoi(radixStr)
	if err != nil || radix < 2 || radix > 62 {
		radix = 36
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return packed
	}

	symtab := strings.Split(symtabStr, "|")
	if len(symtab) < count {
		// pad symtab if needed
		for len(symtab) < count {
			symtab = append(symtab, "")
		}
	}

	// build token replacement map
	var tokenForIndex func(c int, a int) string
	tokenForIndex = func(c int, a int) string {
		var res string
		var encodeChar func(n int) string
		encodeChar = func(n int) string {
			if n > 35 {
				// Base62: 36 -> 'a' (97) or 'A' (65)
				// Dean Edwards: c > 35 ? String.fromCharCode(c+29) : c.toString(36)
				// 36+29 = 65 ('A'), 61+29 = 90 ('Z')
				return string(rune(n + 29))
			}
			return strconv.FormatInt(int64(n), 36)
		}

		var b strings.Builder
		if c >= a {
			b.WriteString(tokenForIndex(c/a, a))
		}
		b.WriteString(encodeChar(c % a))
		res = b.String()
		return res
	}

	// Tokenize payload words and replace
	wordRegex := regexp.MustCompile(`\b\w+\b`)
	replacer := wordRegex.ReplaceAllStringFunc(payload, func(token string) string {
		// convert token back to index
		for i := 0; i < count; i++ {
			if i < len(symtab) && symtab[i] != "" {
				if tokenForIndex(i, radix) == token {
					return symtab[i]
				}
			}
		}
		return token
	})

	return replacer
}

// ExtractImageURLsFromJS extracts image URLs from unpacked JS script
func ExtractImageURLsFromJS(script string) []string {
	// Patterns like: var pvalue = ["/1_1743.jpg", "/2_6582.jpg"]
	// or var pix = "https://image.mangabz.com/1/139/418076"
	var urls []string

	// Look for array patterns: ["...", "..."]
	arrayRegex := regexp.MustCompile(`\[\s*("[^"]+"(?:,\s*"[^"]+")*)\s*\]`)
	pixRegex := regexp.MustCompile(`(?:pix|url|path|domain)\s*=\s*"([^"]+)"`)
	fullURLRegex := regexp.MustCompile(`https?://[^\s"',\[\]\)\(]+\.(?:jpg|png|webp|jpeg)[^\s"',\[\]\)\(]*`)

	fullMatches := fullURLRegex.FindAllString(script, -1)
	if len(fullMatches) > 0 {
		seen := make(map[string]bool)
		for _, u := range fullMatches {
			u = strings.ReplaceAll(u, `\`, "")
			if !seen[u] {
				seen[u] = true
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			return urls
		}
	}

	pixMatch := pixRegex.FindStringSubmatch(script)
	var prefix string
	if len(pixMatch) > 1 {
		prefix = pixMatch[1]
	}

	arrayMatch := arrayRegex.FindStringSubmatch(script)
	if len(arrayMatch) > 1 {
		parts := strings.Split(arrayMatch[1], ",")
		for _, p := range parts {
			clean := strings.Trim(strings.TrimSpace(p), `"`)
			if clean == "" {
				continue
			}
			var fullURL string
			if strings.HasPrefix(clean, "http://") || strings.HasPrefix(clean, "https://") {
				fullURL = clean
			} else if prefix != "" {
				if strings.HasSuffix(prefix, "/") && strings.HasPrefix(clean, "/") {
					fullURL = prefix + clean[1:]
				} else if !strings.HasSuffix(prefix, "/") && !strings.HasPrefix(clean, "/") {
					fullURL = prefix + "/" + clean
				} else {
					fullURL = prefix + clean
				}
			} else {
				fullURL = clean
			}
			urls = append(urls, fullURL)
		}
	}

	return urls
}

// ExtractImagesFromJS is an alias for ExtractImageURLsFromJS
func ExtractImagesFromJS(script string) []string {
	return ExtractImageURLsFromJS(script)
}

func init() {
	_ = fmt.Sprintf("")
}
