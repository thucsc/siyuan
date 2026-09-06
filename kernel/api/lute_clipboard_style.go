// SiYuan - From thought to insight, with agents
// Copyright (c) 2020-present, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"strings"

	"github.com/88250/lute/parse"
	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
)

var clipboardThemeProperties = []string{"color", "background-color", "font-family", "font-size"}

// normalizeBrowserClipboardStyle 移除浏览器复制正文时附带的共同主题样式，保留局部格式差异。
func normalizeBrowserClipboardStyle(dom string) string {
	lower := strings.ToLower(dom)
	// 文档编辑器的样式由作者指定，不按网页正文的主题基准清理。
	for _, marker := range []string{"mso-", "class=\"mso", "urn:schemas-microsoft-com:office", "data-lark", "data-type="} {
		if strings.Contains(lower, marker) {
			return dom
		}
	}
	if !strings.Contains(lower, "-webkit-text-stroke-width") {
		return dom
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(dom))
	if err != nil {
		return dom
	}
	var roots []*nethtml.Node
	var baseline map[string]string
	ambiguous := false
	var collect func(*nethtml.Node)
	collect = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode {
			switch n.Data {
			case "p", "div", "span", "body":
				decl := parse.HTMLStyleDeclarations(clipboardNodeStyle(n))
				if isBrowserClipboardTheme(decl) {
					if baseline == nil {
						baseline = decl
					} else {
						for _, key := range clipboardThemeProperties {
							if decl[key] != baseline[key] {
								ambiguous = true
							}
						}
					}
					roots = append(roots, n)
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(doc.Find("body").Get(0))
	if len(roots) == 0 || ambiguous {
		return dom
	}
	var clean func(*nethtml.Node, map[string]string)
	clean = func(n *nethtml.Node, inherited map[string]string) {
		decl := parse.HTMLStyleDeclarations(clipboardNodeStyle(n))
		effective := make(map[string]string, len(clipboardThemeProperties))
		remove := map[string]bool{}
		var resets strings.Builder
		for _, key := range clipboardThemeProperties {
			effective[key] = inherited[key]
			if value := decl[key]; value != "" && value != "inherit" && value != "unset" {
				effective[key] = value
				if value == baseline[key] {
					remove[key] = true
					// 局部格式之后恢复正文基准时，显式重置，避免继续继承局部格式。
					if inherited[key] != "" && inherited[key] != baseline[key] {
						resets.WriteString(key + ": initial;")
					}
				}
			}
		}
		for i := range n.Attr {
			if n.Attr[i].Key == "style" && len(remove) > 0 {
				n.Attr[i].Val = filterClipboardStyle(n.Attr[i].Val, remove) + resets.String()
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			clean(child, effective)
		}
	}
	for _, root := range roots {
		clean(root, nil)
	}
	ret, err := doc.Find("body").Html()
	if err != nil {
		return dom
	}
	return ret
}

func clipboardNodeStyle(n *nethtml.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "style" {
			return attr.Val
		}
	}
	return ""
}

func isBrowserClipboardTheme(decl map[string]string) bool {
	// 同时要求浏览器序列化特征及完整正文基准，避免把普通手写样式识别为页面主题。
	for _, key := range []string{"-webkit-text-stroke-width", "font-variant-caps", "letter-spacing", "orphans", "widows", "text-indent", "text-transform", "word-spacing", "white-space", "color", "background-color", "font-family", "font-size"} {
		if decl[key] == "" {
			return false
		}
	}
	return true
}

// filterClipboardStyle 仅移除目标声明，原样保留其他声明的优先级、引号和函数内容。
func filterClipboardStyle(style string, remove map[string]bool) string {
	var ret strings.Builder
	start, depth := 0, 0
	var quote rune
	escaped := false
	style += ";"
	for i, c := range style {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '(' {
			depth++
		} else if c == ')' {
			depth--
		}
		if c != ';' || depth != 0 {
			continue
		}
		part := style[start : i+1]
		start = i + 1
		decl := parse.HTMLStyleDeclarations(part)
		keep := true
		for key := range decl {
			if remove[key] {
				keep = false
			}
		}
		if keep && strings.TrimSpace(part) != ";" {
			ret.WriteString(part)
		}
	}
	if start < len(style)-1 {
		ret.WriteString(style[start : len(style)-1])
	}
	return ret.String()
}
