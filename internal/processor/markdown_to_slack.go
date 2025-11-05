package processor

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// MarkdownToSlackProcessor converts a Markdown string to a Slack mrkdwn string.
type MarkdownToSlackProcessor struct {
	htmlProcessor *MarkdownToHTMLProcessor
}

// NewMarkdownToSlackProcessor creates a new MarkdownToSlackProcessor.
func NewMarkdownToSlackProcessor() *MarkdownToSlackProcessor {
	return &MarkdownToSlackProcessor{
		htmlProcessor: NewMarkdownToHTMLProcessor(),
	}
}

// Process converts a Markdown string to a Slack mrkdwn string.
func (p *MarkdownToSlackProcessor) Process(content string, data map[string]interface{}) (string, error) {
	htmlContent, err := p.htmlProcessor.Process(content, data)
	if err != nil {
		return "", err
	}
	return HTMLToMrkdwn(htmlContent)
}

func HTMLToMrkdwn(htmlStr string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}

		if n.Type == html.ElementNode {
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

		if n.Type == html.ElementNode {
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
