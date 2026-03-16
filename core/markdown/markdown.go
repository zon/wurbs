package markdown

import (
	"bytes"
	"html"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
)

func GetMdConverter() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			goldmarkHtml.WithHardWraps(),
		),
	)
}

func ToHTML(md string) (string, error) {
	text := strings.ReplaceAll(md, "<br>", "\n")
	text = html.UnescapeString(text)
	var result bytes.Buffer
	err := GetMdConverter().Convert([]byte(text), &result)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}
