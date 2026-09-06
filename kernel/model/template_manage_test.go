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
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/88250/lute/ast"
	"github.com/88250/lute/parse"
	"github.com/siyuan-note/siyuan/kernel/conf"
	"github.com/siyuan-note/siyuan/kernel/filesys"
	"github.com/siyuan-note/siyuan/kernel/treenode"
	"github.com/siyuan-note/siyuan/kernel/util"
)

func TestTemplateFileManagement(t *testing.T) {
	previous := util.DataDir
	util.DataDir = t.TempDir()
	t.Cleanup(func() { util.DataDir = previous })
	call := func(request TemplateFileRequest) any {
		t.Helper()
		ret, err := ManageTemplateFiles(request)
		if err != nil {
			t.Fatalf("%s %s: %v", request.Action, request.Path, err)
		}
		return ret
	}
	call(TemplateFileRequest{Action: "mkdir", Path: "weekly"})
	call(TemplateFileRequest{Action: "write", Path: "weekly/week.md", Content: "原始模板\r\n"})
	read := call(TemplateFileRequest{Action: "read", Path: "weekly/week.md"}).(map[string]string)
	if read["content"] != "原始模板\r\n" {
		t.Fatal("source bytes changed")
	}
	if _, err := ManageTemplateFiles(TemplateFileRequest{Action: "write", Path: "weekly/week.md", Content: "overwrite"}); err == nil {
		t.Fatal("existing file overwritten without revision")
	}
	updated := call(TemplateFileRequest{Action: "write", Path: "weekly/week.md", Content: "updated", Revision: read["revision"]}).(map[string]string)
	if _, err := ManageTemplateFiles(TemplateFileRequest{Action: "remove", Path: "weekly/week.md", Revision: read["revision"]}); err == nil {
		t.Fatal("stale delete accepted")
	}
	call(TemplateFileRequest{Action: "move", Path: "weekly/week.md", Target: "renamed.md", Revision: updated["revision"]})
	if _, err := os.Stat(filepath.Join(util.DataDir, "templates", "weekly", "week.md")); !os.IsNotExist(err) {
		t.Fatal("source still exists")
	}
	call(TemplateFileRequest{Action: "write", Path: "weekly/child.md", Content: "child"})
	dir := call(TemplateFileRequest{Action: "read", Path: "weekly"}).(map[string]string)
	deleted := call(TemplateFileRequest{Action: "remove", Path: "weekly", Revision: dir["revision"]}).(map[string]string)
	content, err := os.ReadFile(filepath.Join(util.DataDir, "templates", deleted["recoveryPath"], "child.md"))
	if err != nil || string(content) != "child" {
		t.Fatalf("deleted directory is not recoverable: %s %v", content, err)
	}
	entries := call(TemplateFileRequest{Action: "list"}).([]TemplateFileEntry)
	if len(entries) != 1 || entries[0].Path != "renamed.md" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestTemplateFilePaths(t *testing.T) {
	previous := util.DataDir
	util.DataDir = t.TempDir()
	t.Cleanup(func() { util.DataDir = previous })
	for _, p := range []string{"", ".", "../outside.md", "/outside.md", "C:/outside.md", "a\\b.md", ".trash/a.md", "a/../b.md", "a:stream.md", "a /b.md"} {
		if _, err := ManageTemplateFiles(TemplateFileRequest{Action: "write", Path: p, Content: "bad"}); err == nil {
			t.Errorf("accepted unsafe path %q", p)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(util.DataDir, "templates", "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ManageTemplateFiles(TemplateFileRequest{Action: "write", Path: "linked/outside.md"}); err == nil {
		t.Fatal("symlink escape accepted")
	}
}

func TestTemplateDocumentAttributes(t *testing.T) {
	icon := `api/icon/getDynamicIcon?type=5&date=.action{now | date "2006-01-02"}`
	attrs := templateDocumentAttributes([]byte(`{: icon="` + icon + `" custom-test=".action{printf "}"}" type="doc"}`))
	if len(attrs) != 3 || attrs[0][1] != icon || attrs[1][1] != `.action{printf "}"}` {
		t.Fatalf("actions were damaged: %#v", attrs)
	}
	for _, source := range []string{`{: icon="x"}`, "{: type=\"doc\"}\nother content", "{: type=\"doc\"}\n{: id=\"x\"}", `{: icon=".action{now" type="doc"}`} {
		if attrs := templateDocumentAttributes([]byte(source)); attrs != nil {
			t.Errorf("accepted non-document declaration %q: %#v", source, attrs)
		}
	}
}

func TestPreviewTemplateUnsavedSource(t *testing.T) {
	fixture := setupFileOperationTest(t)
	p := writeTemplateDocTreeTestFile(t, "saved content")
	_, dom, _, err := PreviewTemplateSource(p, fixture.sourceID, "unsaved .action{.id}")
	if err != nil || !strings.Contains(dom, "unsaved "+fixture.sourceID) || strings.Contains(dom, "saved content") {
		t.Fatalf("unexpected preview: %s %v", dom, err)
	}
	content, err := os.ReadFile(p)
	if err != nil || string(content) != "saved content" {
		t.Fatal("preview wrote to template file")
	}
}

func TestExportTemplateDocumentAttributesAndDirectory(t *testing.T) {
	fixture := setupFileOperationTest(t)
	Conf.Editor = conf.NewEditor()
	Conf.Export = conf.NewExport()
	tree, err := LoadTreeByBlockID(fixture.sourceID)
	if err != nil {
		t.Fatal(err)
	}
	tree.Root.SetIALAttr("icon", "old-icon")
	tree.Root.SetIALAttr("custom-keep", "retained")
	engine := NewLute()
	codeTree := parse.Parse("", []byte("```template\n{: icon=\"api/icon/getDynamicIcon?type=5&date=.action{now | date \"2006-01-02\"}\" type=\"doc\"}\n```\n\nAfter declaration\n"), engine.ParseOptions)
	var codeID string
	for child := codeTree.Root.FirstChild; child != nil; {
		next := child.Next
		child.ID = ast.NewNodeID()
		child.SetIALAttr("id", child.ID)
		if child.Type == ast.NodeCodeBlock {
			codeID = child.ID
		}
		tree.Root.AppendChild(child)
		child = next
	}
	if _, err = filesys.WriteTree(tree); err != nil {
		t.Fatal(err)
	}
	treenode.UpsertBlockTree(tree)
	if _, err = ManageTemplateFiles(TemplateFileRequest{Action: "mkdir", Path: "weekly"}); err != nil {
		t.Fatal(err)
	}
	code, err := DocSaveAsTemplateInDirectory(fixture.sourceID, "week", "weekly", false, TemplateDatabaseModeCopy)
	if code != 0 || err != nil {
		t.Fatalf("export failed: %d %v", code, err)
	}
	p := filepath.Join(util.DataDir, "templates", "weekly", "week.md")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(content), `type="doc"`) != 1 || strings.Contains(string(content), codeID) || !strings.Contains(string(content), "After declaration") {
		t.Fatalf("unexpected exported attributes/content: %s", content)
	}
	rendered, _, _, err := RenderTemplateWithMode(p, fixture.sourceID, TemplateRenderModePreview)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rendered.Root.IALAttr("icon"), "api/icon/getDynamicIcon?type=5&date=") || strings.Contains(rendered.Root.IALAttr("icon"), ".action{") || rendered.Root.IALAttr("custom-keep") != "retained" {
		t.Fatalf("document attributes were not merged: %#v", rendered.Root.KramdownIAL)
	}
	code, err = DocSaveAsTemplateInDirectory(fixture.sourceID, "week", "weekly", false, TemplateDatabaseModeCopy)
	if code != 1 || err != nil {
		t.Fatalf("overwrite confirmation missing: %d %v", code, err)
	}
	if _, err = DocSaveAsTemplateInDirectory(fixture.sourceID, "week", "../escape", true, TemplateDatabaseModeCopy); err == nil {
		t.Fatal("export path escaped templates")
	}
}
