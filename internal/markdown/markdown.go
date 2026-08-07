package markdown

import (
	"bytes"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	gm = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)
	policy = bluemonday.UGCPolicy()
)

func Render(md string) (string, error) {
	if md == "" {
		return "", nil
	}
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return policy.SanitizeReader(&buf).String(), nil
}
