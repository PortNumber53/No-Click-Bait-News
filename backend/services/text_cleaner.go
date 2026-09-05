package services

import (
	"strings"

	"golang.org/x/net/html"
)

var htmlLineBreakElements = map[string]bool{
	"address":    true,
	"article":    true,
	"aside":      true,
	"blockquote": true,
	"br":         true,
	"div":        true,
	"figcaption": true,
	"figure":     true,
	"footer":     true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"header":     true,
	"li":         true,
	"main":       true,
	"nav":        true,
	"p":          true,
	"section":    true,
	"tr":         true,
}

// stripHTMLMarkup removes HTML elements while retaining their readable text.
// Script and style contents are discarded. Line-oriented elements become line
// breaks so adjacent paragraphs do not run together.
func stripHTMLMarkup(value string) string {
	if value == "" {
		return ""
	}

	tokenizer := html.NewTokenizer(strings.NewReader(value))
	var output strings.Builder
	skipDepth := 0

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return normalizeHTMLText(output.String())
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			tag := strings.ToLower(string(name))
			if tag == "script" || tag == "style" {
				skipDepth++
				continue
			}
			if skipDepth == 0 && htmlLineBreakElements[tag] {
				appendLineBreak(&output)
			}
		case html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			if skipDepth == 0 && htmlLineBreakElements[strings.ToLower(string(name))] {
				appendLineBreak(&output)
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			tag := strings.ToLower(string(name))
			if tag == "script" || tag == "style" {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			if skipDepth == 0 && htmlLineBreakElements[tag] {
				appendLineBreak(&output)
			}
		case html.TextToken:
			if skipDepth == 0 {
				output.Write(tokenizer.Text())
			}
		}
	}
}

func appendLineBreak(output *strings.Builder) {
	if output.Len() == 0 {
		return
	}
	value := output.String()
	if value[len(value)-1] != '\n' {
		output.WriteByte('\n')
	}
}

func normalizeHTMLText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	previousBlank := true
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !previousBlank {
				cleaned = append(cleaned, "")
				previousBlank = true
			}
			continue
		}
		cleaned = append(cleaned, line)
		previousBlank = false
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
