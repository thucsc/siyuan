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
	"regexp"
	"strings"

	"github.com/88250/lute/parse"
	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
)

var clipboardThemeProperties = []string{"color", "background-color", "font-family", "font-size"}

var clipboardTransparentColor = regexp.MustCompile(`(?i)^(transparent|#[0-9a-f]{3}0|#[0-9a-f]{6}00|(?:rgba|hsla)\([^)]*,\s*(?:0(?:\.0*)?|\.0+)%?\s*\)|(?:rgb|rgba|hsl|hsla)\([^)]*/\s*(?:0(?:\.0*)?|\.0+)%?\s*\))$`)

// normalizeBrowserClipboardStyle 移除浏览器复制正文时附带的共同主题样式，保留局部格式差异。
func normalizeBrowserClipboardStyle(dom string, documentSource bool) string {
	lower := strings.ToLower(dom)
	if strings.Contains(lower, "data-type=") {
		return dom
	}
	if !strings.Contains(lower, "style") {
		return dom
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(dom))
	if err != nil {
		return dom
	}
	doc.Find("*").Each(func(_ int, selection *goquery.Selection) {
		if isClipboardDocumentBoundary(selection.Get(0)) {
			documentSource = true
		}
	})
	changed := false
	if !documentSource {
		doc.Find("[style]").Each(func(_ int, selection *goquery.Selection) {
			decl := parse.HTMLStyleDeclarations(clipboardNodeStyle(selection.Get(0)))
			changed = changed || clipboardTransparentColor.MatchString(decl["background-color"])
		})
		// 网页链接自身的外观交给编辑器，子元素中的显式格式及语义标签继续保留。
		doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
			n := selection.Get(0)
			for i := range n.Attr {
				if n.Attr[i].Key != "style" {
					continue
				}
				style := n.Attr[i].Val
				remove := map[string]bool{"color": true, "background-color": true, "font-family": true, "font-size": true}
				decl := parse.HTMLStyleDeclarations(style)
				var retained strings.Builder
				for _, key := range []string{"text-decoration", "text-decoration-line"} {
					var parts []string
					for _, part := range strings.Fields(decl[key]) {
						if strings.EqualFold(part, "underline") {
							remove[key] = true
						} else {
							parts = append(parts, part)
						}
					}
					if remove[key] && len(parts) > 0 {
						retained.WriteString(key + ": " + strings.Join(parts, " ") + ";")
					}
				}
				n.Attr[i].Val = filterClipboardStyle(style, remove) + retained.String()
				changed = changed || n.Attr[i].Val != style
			}
		})
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
				if isBrowserClipboardTheme(decl) && (!documentSource || isClipboardDocumentThemeWrapper(n)) {
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
			// 文档正文内部的统一字体、字号也可能是作者主动设置，不能用重复次数推断默认格式。
			if documentSource && isClipboardDocumentBoundary(n) {
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(doc.Find("body").Get(0))
	if len(roots) == 0 || ambiguous {
		roots = nil
		if !changed {
			return dom
		}
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
		// 文档来源只清理外层浏览器包装；即使子元素与主题同色，也保留其显式格式。
		if documentSource {
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			clean(child, effective)
		}
	}
	for _, root := range roots {
		clean(root, nil)
	}
	if !documentSource {
		// 透明背景不生成文本样式；存在祖先背景时保留重置语义，避免被文本样式继承重新着色。
		doc.Find("[style]").Each(func(_ int, selection *goquery.Selection) {
			n := selection.Get(0)
			for i := range n.Attr {
				if n.Attr[i].Key != "style" {
					continue
				}
				decl := parse.HTMLStyleDeclarations(n.Attr[i].Val)
				if !clipboardTransparentColor.MatchString(decl["background-color"]) {
					continue
				}
				n.Attr[i].Val = filterClipboardStyle(n.Attr[i].Val, map[string]bool{"background-color": true})
				for parent := n.Parent; parent != nil; parent = parent.Parent {
					if parse.HTMLStyleDeclarations(clipboardNodeStyle(parent))["background-color"] != "" {
						n.Attr[i].Val += "background-color: initial;"
						break
					}
				}
			}
		})
	}
	ret, err := doc.Find("body").Html()
	if err != nil {
		return dom
	}
	return ret
}

func isClipboardDocumentBoundary(n *nethtml.Node) bool {
	for _, attr := range n.Attr {
		key, value := strings.ToLower(attr.Key), strings.ToLower(attr.Val)
		if strings.HasPrefix(key, "data-lark") || strings.Contains(value, "urn:schemas-microsoft-com:office") {
			return true
		}
		if key == "style" && strings.Contains(value, "mso-") {
			return true
		}
		if key == "class" {
			for _, class := range strings.Fields(value) {
				if strings.HasPrefix(class, "mso") {
					return true
				}
			}
		}
	}
	return false
}

func isClipboardDocumentThemeWrapper(n *nethtml.Node) bool {
	if n.Data != "div" && n.Data != "body" {
		return false
	}
	if isClipboardDocumentBoundary(n) {
		// 飞书仅允许根容器参与浏览器样式识别；正文节点和 Office 样式节点不作为主题基准。
		larkRoot := false
		for _, attr := range n.Attr {
			if attr.Key == "data-lark-html-role" && attr.Val == "root" {
				larkRoot = true
			}
			if attr.Key == "style" && strings.Contains(strings.ToLower(attr.Val), "mso-") {
				return false
			}
		}
		if !larkRoot {
			return false
		}
	}
	hasContent := false
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == nethtml.TextNode && strings.TrimSpace(child.Data) != "" {
			return false
		}
		if child.Type == nethtml.ElementNode {
			hasContent = true
		}
	}
	return hasContent
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
