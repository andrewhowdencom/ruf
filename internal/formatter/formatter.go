package formatter

import (
	"bytes"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	nethtml "golang.org/x/net/html"
)

// ToHTML converts a Markdown string to an HTML string.
func ToHTML(md []byte) []byte {
	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}

// ToSlack converts a Markdown string to a Slack mrkdwn string.
func ToSlack(md []byte) (string, error) {
	html := ToHTML(md)
	return htmlToMrkdwn(string(html))
}

func htmlToMrkdwn(htmlStr string) (string, error) {
	doc, err := nethtml.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	var traverse func(*nethtml.Node)
	traverse = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			buf.WriteString(n.Data)
		}

		if n.Type == nethtml.ElementNode {
			switch n.Data {
			case "p":
				buf.WriteString("\n")
			case "h1", "h2", "h3", "h4", "h5", "h6":
				buf.WriteString("*")
			case "a":
				var href string
				for _, a := range n.Attr {
					if a.Key == "href" {
						href = a.Val
						break
					}
				}
				buf.WriteString("<" + href + "|")
			case "li":
				if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
					buf.WriteString("\n")
				}
				buf.WriteString("• ")
			case "strong", "b":
				buf.WriteString("*")
			case "em", "i":
				buf.WriteString("_")
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

		if n.Type == nethtml.ElementNode {
			switch n.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				buf.WriteString("*")
			case "a":
				buf.WriteString(">")
			case "strong", "b":
				buf.WriteString("*")
			case "em", "i":
				buf.WriteString("_")
			}
		}
	}

	traverse(doc)
	return strings.TrimSpace(buf.String()), nil
}
