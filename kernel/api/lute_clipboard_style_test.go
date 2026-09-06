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
	output := normalizeBrowserClipboardStyle(input)
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
	if again := normalizeBrowserClipboardStyle(output); again != output {
		t.Fatal("normalization is not idempotent")
	}
}

func TestBrowserClipboardLocalStyles(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(githubClipboardFixture(t)))
	style, _ := doc.Find("p").First().Attr("style")
	input := `<div style='` + style + `'>plain <strong>bold</strong><em>italic</em><a href="https://example.com">link</a>` +
		`<span style="color: red; background: yellow; font-family: serif; font-size: 20px">highlight` +
		`<span style="color: rgb(240, 246, 252)">reset</span></span><small>small</small><code>code</code></div>`
	output := normalizeBrowserClipboardStyle(input)
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
			output := normalizeBrowserClipboardStyle(input)
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
		if output := normalizeBrowserClipboardStyle(input); output != input {
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
