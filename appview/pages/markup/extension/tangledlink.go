package extension

import (
	"net/url"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindTangledLink is a NodeKind of the TangledLink node.
var KindTangledLink = ast.NewNodeKind("TangledLink")

type TangledLinkNode struct {
	ast.BaseInline
	Destination string
	Commit      *TangledCommitLink
	// TODO: add more Tangled-link types
}

type TangledCommitLink struct {
	Sha string
}

var _ ast.Node = new(TangledLinkNode)

// Dump implements [ast.Node].
func (n *TangledLinkNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

// Kind implements [ast.Node].
func (n *TangledLinkNode) Kind() ast.NodeKind {
	return KindTangledLink
}

type tangledLinkTransformer struct {
	host string
}

var _ parser.ASTTransformer = new(tangledLinkTransformer)

// Transform implements [parser.ASTTransformer].
func (t *tangledLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		var dest string

		switch n := n.(type) {
		case *ast.AutoLink:
			dest = string(n.URL(reader.Source()))
		case *ast.Link:
			// maybe..? not sure
		default:
			return ast.WalkContinue, nil
		}

		if sha := t.parseLinkCommitSha(dest); sha != "" {
			newLink := &TangledLinkNode{
				Destination: dest,
				Commit: &TangledCommitLink{
					Sha: sha,
				},
			}
			n.Parent().ReplaceChild(n.Parent(), n, newLink)
		}

		return ast.WalkContinue, nil
	})
}

func (t *tangledLinkTransformer) parseLinkCommitSha(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "tangled.org" {
		return ""
	}

	// /{owner}/{repo}/commit/<sha>
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "commit" {
		return ""
	}

	sha := parts[3]

	// basic sha validation
	if len(sha) < 7 {
		return ""
	}
	for _, c := range sha {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return ""
		}
	}

	return sha[:8]
}

type tangledLinkRenderer struct{}

var _ renderer.NodeRenderer = new(tangledLinkRenderer)

// RegisterFuncs implements [renderer.NodeRenderer].
func (r *tangledLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindTangledLink, r.renderTangledLink)
}

func (r *tangledLinkRenderer) renderTangledLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	link := node.(*TangledLinkNode)

	if link.Commit != nil {
		if entering {
			w.WriteString(`<a href="`)
			w.WriteString(link.Destination)
			w.WriteString(`"><code>`)
			w.WriteString(link.Commit.Sha)
		} else {
			w.WriteString(`</code></a>`)
		}
	}

	return ast.WalkContinue, nil
}

type tangledLinkExt struct {
	host string
}

var _ goldmark.Extender = new(tangledLinkExt)

func (e *tangledLinkExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(&tangledLinkTransformer{host: e.host}, 500),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&tangledLinkRenderer{}, 500),
	))
}

func NewTangledLinkExt(host string) goldmark.Extender {
	return &tangledLinkExt{host}
}
