package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToHTML(t *testing.T) {
	md := "a\nb\nc"
	html := "<p>a<br>\nb<br>\nc</p>\n"
	res, err := ToHTML(md)
	assert.NoError(t, err)
	assert.Equal(t, html, res)
}

func TestToHTMLCode(t *testing.T) {
	md := "```\na\nb\nc\n```"
	html := "<pre><code>a\nb\nc\n</code></pre>\n"
	res, err := ToHTML(md)
	assert.NoError(t, err)
	assert.Equal(t, html, res)
}

func TestGetMdConverter(t *testing.T) {
	conv := GetMdConverter()
	assert.NotNil(t, conv)
}
