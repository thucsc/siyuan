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
	"os"
	"strings"
	"testing"

	"github.com/88250/lute/parse"
	"github.com/88250/lute/render"
	"github.com/PuerkitoBio/goquery"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func githubClipboardFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/github-theme-clipboard.html")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBrowserClipboardTheme(t *testing.T) {
	input := render.Sanitize(githubClipboardFixture(t))
	output := normalizeBrowserClipboardStyle(input, false)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(output))
	doc.Find("p").Each(func(_ int, p *goquery.Selection) {
		style, _ := p.Attr("style")
		decl := parse.HTMLStyleDeclarations(style)
		for _, key := range clipboardThemeProperties {
			if decl[key] != "" {
				t.Errorf("theme property %s survived: %s", key, style)
			}
		}
	})
	lute := util.NewLute()
	lute.SetHTMLTag2TextMark(true)
	blockDOM := lute.HTML2BlockDOM(output)
	if strings.Contains(blockDOM, "rgb(") || strings.Contains(blockDOM, "font-size:") || strings.Contains(blockDOM, "font-family:") {
		t.Fatalf("theme leaked into BlockDOM: %s", blockDOM)
	}
	if !strings.Contains(blockDOM, "感谢分享经验") || !strings.Contains(blockDOM, "欢迎提供") {
		t.Fatalf("content lost: %s", blockDOM)
	}
	if again := normalizeBrowserClipboardStyle(output, false); again != output {
		t.Fatal("normalization is not idempotent")
	}
}

func TestBrowserClipboardLocalStyles(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	style, _ := doc.Find("p").First().Attr("style")
	input := `<div style='` + style + `'>plain <strong>bold</strong><em>italic</em><a href="https://example.com">link</a>` +
		`<span style="color: red; background: yellow; font-family: serif; font-size: 20px">highlight` +
		`<span style="color: rgb(240, 246, 252)">reset</span></span><small>small</small><code>code</code></div>`
	output := normalizeBrowserClipboardStyle(input, false)
	lute := util.NewLute()
	lute.SetHTMLTag2TextMark(true)
	blockDOM := lute.HTML2BlockDOM(output)
	for _, expected := range []string{"color: red", "background-color: yellow", "font-family: serif", "font-size: 20.000000px", "strong", "em", "https://example.com", "code", "font-size: 0.833333em"} {
		if !strings.Contains(blockDOM, expected) {
			t.Errorf("missing %q in %s", expected, blockDOM)
		}
	}
	if strings.Contains(blockDOM, "rgb(240") || strings.Contains(blockDOM, "rgb(13") {
		t.Fatalf("theme survived nested reset: %s", blockDOM)
	}
}

func TestBrowserClipboardThemeVariants(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	style, _ := doc.Find("p").First().Attr("style")
	for _, tag := range []string{"p", "span", "div"} {
		for _, light := range []bool{false, true} {
			baseline := style
			if light {
				baseline = strings.ReplaceAll(baseline, "rgb(240, 246, 252)", "rgb(31, 35, 40)")
				baseline = strings.ReplaceAll(baseline, "rgb(13, 17, 23)", "rgb(255, 255, 255)")
			}
			input := "<" + tag + " style='" + baseline + "'>plain<span style='" + baseline + "'>repeated</span></" + tag + ">"
			output := normalizeBrowserClipboardStyle(input, false)
			lute := util.NewLute()
			lute.SetHTMLTag2TextMark(true)
			blockDOM := lute.HTML2BlockDOM(output)
			if strings.Contains(blockDOM, "rgb(") || strings.Contains(blockDOM, "font-size:") || strings.Contains(blockDOM, "font-family:") || !strings.Contains(blockDOM, "plainrepeated") {
				t.Errorf("theme variant %s light=%v: %s", tag, light, blockDOM)
			}
		}
	}
}

func TestBrowserClipboardPreservesExplicitStyles(t *testing.T) {
	fixture := githubClipboardFixture(t)
	for _, input := range []string{
		`<p style="color: white; background-color: black; font-family: serif; font-size: 14px">explicit</p>`,
		`<div data-lark-html-role="root">` + fixture + `</div>`,
		`<div style="mso-style-name: normal">` + fixture + `</div>`,
		`<div data-type="NodeParagraph">` + fixture + `</div>`,
		strings.Replace(fixture, "rgb(240, 246, 252)", "red", 1),
	} {
		if output := normalizeBrowserClipboardStyle(input, false); output != input {
			t.Errorf("explicit or ambiguous formatting changed: %s", output)
		}
	}
	noMath := func(string, string, string) (string, bool) { return "", false }
	noOffice := func(string) (string, bool) { return "", false }
	normalized, _ := prepareHTMLClipboardContent(util.NewLute(), fixture, "", "", "", "", "", false, noMath, noOffice)
	if strings.Contains(normalized, "rgb(240, 246, 252)") {
		t.Fatal("clipboard preparation did not normalize browser styles")
	}
	frozen, _ := prepareHTMLClipboardContent(util.NewLute(), normalized, "", "", "", "", "", true, noMath, noOffice)
	if frozen != normalized {
		t.Fatal("normalized HTML changed during the second conversion stage")
	}
	for _, office := range [][3]string{{"office", "", ""}, {"", "officeHTML", ""}, {"", "", "wps"}} {
		output, _ := prepareHTMLClipboardContent(util.NewLute(), fixture, "", "", office[0], office[1], office[2], false, noMath, noOffice)
		if !strings.Contains(output, "rgb(240, 246, 252)") {
			t.Fatal("Office theme-like formatting was removed")
		}
	}
	prepared, _ := prepareHTMLClipboardContent(util.NewLute(), fixture, "", "", "", "", "", true, noMath, noOffice)
	if prepared != fixture {
		t.Fatal("prepared HTML was processed again")
	}
}

func TestFilterClipboardStyle(t *testing.T) {
	input := `color: red !important; color: blue; font-family: "a;b"; background-image: url("a;b"); margin: 0 !important;`
	output := filterClipboardStyle(input, map[string]bool{"color": true})
	if strings.Contains(output, "color:") || !strings.Contains(output, `font-family: "a;b";`) || !strings.Contains(output, `background-image: url("a;b");`) || !strings.Contains(output, "margin: 0 !important;") {
		t.Fatal(output)
	}
}

func TestDocumentClipboardThemeWrapper(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	theme, _ := doc.Find("p").First().Attr("style")
	noMath := func(string, string, string) (string, bool) { return "", false }
	noOffice := func(string) (string, bool) { return "", false }
	for _, tc := range []struct {
		name, root, content, office, officeHTML, wps string
	}{
		{"wps", "", `<p class=MsoNormal><span style="font-family:宋体;font-size:12pt">正文</span><u><span style="color:red;background:yellow">强调</span></u></p>`, "", "", "wps"},
		{"office", "", `<p class='MsoNormal'><span style="font-family:Calibri;font-size:11pt">正文</span><strong>强调</strong></p>`, "office", "", ""},
		{"officeHTML", "", `<p><span style="font-family:Calibri;font-size:11pt">正文</span></p>`, "", "officeHTML", ""},
		{"feishu", `data-lark-html-role="root"`, `<div>正文<span style="color:red;background-color:yellow">强调</span></div>`, "", "", ""},
		{"officeDetected", "", `<p class='MsoNormal'><span style="font-family:Calibri;font-size:11pt">正文</span></p>`, "", "", ""},
		{"sameAsTheme", `data-lark-html-role="root"`, `<p><span style='` + theme + `'>显式格式</span></p>`, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `<div id="wrapper" ` + tc.root + ` style='` + theme + `'>` + tc.content + `</div>`
			output, useHTML := prepareHTMLClipboardContent(util.NewLute(), input, "", "", tc.office, tc.officeHTML, tc.wps, false, noMath, noOffice)
			if !useHTML {
				t.Fatal("HTML conversion was skipped")
			}
			actual, _ := goquery.NewDocumentFromReader(strings.NewReader(output))
			wrapper := actual.Find("#wrapper")
			style, _ := wrapper.Attr("style")
			decl := parse.HTMLStyleDeclarations(style)
			for _, key := range clipboardThemeProperties {
				if decl[key] != "" {
					t.Errorf("wrapper retained %s: %s", key, output)
				}
			}
			before, _ := goquery.NewDocumentFromReader(strings.NewReader(input))
			want, _ := before.Find("#wrapper").Html()
			got, _ := wrapper.Html()
			if got != want {
				t.Errorf("explicit content formatting changed:\nwant %s\ngot %s", want, got)
			}
			if again := normalizeBrowserClipboardStyle(output, tc.office != "" || tc.officeHTML != "" || tc.wps != ""); again != output {
				t.Fatal("document normalization is not idempotent")
			}
		})
	}
}

func TestDocumentClipboardAmbiguousStyles(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	theme, _ := doc.Find("p").First().Attr("style")
	for _, input := range []string{
		`<div style='` + theme + `'>direct text</div>`,
		`<div class='MsoNormal' style='` + theme + `'><span>text</span></div>`,
		`<div style='` + theme + `mso-style-name:Normal'><span>text</span></div>`,
		`<div data-lark-html-role="paragraph" style='` + theme + `'><span>text</span></div>`,
		`<div><span style="font-family:宋体;font-size:12pt">text</span></div>`,
	} {
		if output := normalizeBrowserClipboardStyle(input, true); output != input {
			t.Errorf("ambiguous document style changed: %s", output)
		}
	}
}

func TestBrowserClipboardLinkAppearance(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	theme, _ := doc.Find("p").First().Attr("style")
	for _, input := range []string{
		`<p>before <a href="https://example.com" title="title" style="color:rgb(9,105,218);background-color:rgba(0,0,0,0);text-decoration:underline">link</a> after</p>`,
		`<a href="https://example.com" title="title" style='` + theme + `text-decoration:underline'>link</a>`,
		`<p style='` + theme + `'>before <a href="https://example.com" title="title" style="color:rgb(9,105,218);text-decoration-line:underline">link</a> after</p>`,
	} {
		output := normalizeBrowserClipboardStyle(input, false)
		lute := util.NewLute()
		lute.SetHTMLTag2TextMark(true)
		blockDOM := lute.HTML2BlockDOM(output)
		if !strings.Contains(blockDOM, `data-type="a"`) || !strings.Contains(blockDOM, `data-href="https://example.com"`) || !strings.Contains(blockDOM, `data-title="title"`) {
			t.Errorf("link semantics lost: %s", blockDOM)
		}
		for _, unexpected := range []string{"color:", "font-family:", "font-size:", `data-type="a u"`} {
			if strings.Contains(blockDOM, unexpected) {
				t.Errorf("link appearance survived %q: %s", unexpected, blockDOM)
			}
		}
		if again := normalizeBrowserClipboardStyle(output, false); again != output {
			t.Fatal("link cleanup is not idempotent")
		}
	}
}

func TestBrowserClipboardLinkExplicitContent(t *testing.T) {
	input := `<a href="https://example.com" style="color:blue;text-decoration:underline line-through"><strong>bold</strong><u>underline</u><span style="color:red;background:yellow;font-size:20px;font-family:serif">styled</span></a>`
	lute := util.NewLute()
	lute.SetHTMLTag2TextMark(true)
	output := lute.HTML2BlockDOM(normalizeBrowserClipboardStyle(input, false))
	for _, want := range []string{"strong", "u", "s", "color: red", "background-color: yellow", "font-size: 20.000000px", "font-family: serif"} {
		if !strings.Contains(output, want) {
			t.Errorf("missing %s: %s", want, output)
		}
	}
	for _, input := range []string{input, `<div data-lark-html-role="root">` + input + `</div>`, `<p class=MsoNormal>` + input + `</p>`} {
		if output := normalizeBrowserClipboardStyle(input, true); output != input {
			t.Errorf("document link changed: %s", output)
		}
	}
}

func TestBrowserClipboardTransparentBackground(t *testing.T) {
	for _, color := range []string{"transparent", "rgba(0, 0, 0, 0)", "rgb(0 0 0 / 0%)", "#1230", "#12345600"} {
		input := `<span style="background-color:` + color + `">plain</span>`
		output := normalizeBrowserClipboardStyle(input, false)
		if strings.Contains(output, "background-color") {
			t.Error(output)
		}
	}
	for _, color := range []string{"#abcd00", "rgba(0,0,0,0.5)", "yellow"} {
		input := `<span style="background-color:` + color + `">highlight</span>`
		if output := normalizeBrowserClipboardStyle(input, false); output != input {
			t.Errorf("visible background changed: %s", output)
		}
	}
	input := `<div style="background-color:yellow"><span style="background-color:rgba(0,0,0,0)">plain</span></div>`
	lute := util.NewLute()
	lute.SetHTMLTag2TextMark(true)
	output := lute.HTML2BlockDOM(normalizeBrowserClipboardStyle(input, false))
	if strings.Contains(output, "background-color:") {
		t.Errorf("transparent text inherited a background: %s", output)
	}
}

func TestBrowserClipboardSemanticAppearance(t *testing.T) {
	style := `color: white; background-color: black; font-family: Arial; font-size: 14px;`
	for _, tc := range []struct {
		tag, content, blockType string
	}{
		{"h2", "heading", "NodeHeading"},
		{"blockquote", "<p>quote</p>", "NodeBlockquote"},
		{"ul", "<li>first<ul><li>nested</li></ul></li>", "NodeList"},
		{"ol", "<li>first</li><li>second</li>", "NodeList"},
		{"code", "inline", "NodeParagraph"},
		{"pre", "<code class=\"language-go\">  first\n\tsecond\n</code>", "NodeCodeBlock"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			input := "<" + tc.tag + " style='" + style + "'>" + tc.content + "</" + tc.tag + ">"
			output := normalizeBrowserClipboardStyle(input, false)
			before, _ := goquery.NewDocumentFromReader(strings.NewReader(input))
			after, _ := goquery.NewDocumentFromReader(strings.NewReader(output))
			want, _ := before.Find(tc.tag).First().Html()
			got, _ := after.Find(tc.tag).First().Html()
			if got != want {
				t.Errorf("semantic content changed: want %q, got %q", want, got)
			}
			lute := util.NewLute()
			lute.SetHTMLTag2TextMark(true)
			blockDOM := lute.HTML2BlockDOM(output)
			if !strings.Contains(blockDOM, tc.blockType) {
				t.Errorf("semantic type lost: %s", blockDOM)
			}
			for _, property := range clipboardThemeProperties {
				if strings.Contains(blockDOM, property+":") {
					t.Errorf("default style %s survived: %s", property, blockDOM)
				}
			}
			if again := normalizeBrowserClipboardStyle(output, false); again != output {
				t.Fatal("semantic normalization is not idempotent")
			}
		})
	}
}

func TestBrowserClipboardSemanticLocalFormatting(t *testing.T) {
	input := `<h2 style="color:black;font-size:24px"><strong>heading</strong><span style="color:red;font-size:30px">emphasis</span></h2>` +
		`<blockquote style="color:gray"><p><u>underline</u><mark>highlight</mark></p></blockquote>` +
		`<ol start="3"><li style="font-family:Arial;color:black"><span style="background:yellow">item</span></li></ol>`
	output := normalizeBrowserClipboardStyle(input, false)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(output))
	if start, _ := doc.Find("ol").Attr("start"); start != "3" || doc.Find("strong,u,mark").Length() != 3 {
		t.Fatal("semantic attributes or emphasis lost: " + output)
	}
	for _, want := range []string{"color:red", "font-size:30px", "background:yellow"} {
		if !strings.Contains(output, want) {
			t.Errorf("local style %s lost: %s", want, output)
		}
	}
	for _, wrapper := range []string{`<div data-lark-html-role="root">`, `<div class="MsoNormal">`, `<div data-type="NodeParagraph">`} {
		protected := wrapper + input + `</div>`
		if actual := normalizeBrowserClipboardStyle(protected, false); actual != protected {
			t.Error("document semantic formatting changed: " + actual)
		}
	}
	if actual := normalizeBrowserClipboardStyle(input, true); actual != input {
		t.Error("Office clipboard semantic formatting changed: " + actual)
	}
}

func TestBrowserClipboardCodeAndTable(t *testing.T) {
	code := "<pre style='background:black'><code class='language-go' style='color:white'><span style='color:red'>  first</span>\n\tsecond\n</code></pre>"
	output := normalizeBrowserClipboardStyle(code, false)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(output))
	if doc.Find("code").Text() != "  first\n\tsecond\n" {
		t.Fatal("code whitespace changed: " + output)
	}
	if language, _ := doc.Find("code").Attr("class"); language != "language-go" || strings.Contains(output, "color:") {
		t.Fatal("code language or coloring changed incorrectly: " + output)
	}
	table := `<table style="font-family:Arial"><thead><tr><th style="font-size:14px;background:gray">header</th></tr></thead><tbody><tr><td rowspan="2" style="color:red;background:yellow;font-size:14px">status</td></tr><tr></tr></tbody></table>`
	output = normalizeBrowserClipboardStyle(table, false)
	doc, _ = goquery.NewDocumentFromReader(strings.NewReader(output))
	if rowspan, _ := doc.Find("td").Attr("rowspan"); rowspan != "2" || doc.Find("th").Text() != "header" {
		t.Fatal("table structure lost: " + output)
	}
	if strings.Contains(output, "font-family") || strings.Contains(output, "font-size") || !strings.Contains(output, "color:red;background:yellow") || !strings.Contains(output, "background:gray") {
		t.Fatal("table appearance changed incorrectly: " + output)
	}
}
