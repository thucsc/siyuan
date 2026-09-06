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
	"bytes"
	"strings"

	"github.com/88250/lute/ast"
	"github.com/88250/lute/parse"
)

// 独立模板代码块中的文档属性合并到导出文档末尾，保留动作源码中的引号。
func templateDocumentAttributes(source []byte) [][]string {
	text := strings.TrimSpace(string(source))
	if !strings.HasPrefix(text, "{:") || !strings.HasSuffix(text, "}") {
		return nil
	}
	prefix := "siyuan_template_" + ast.NewNodeID() + "_"
	var actions []string
	var masked strings.Builder
	for {
		start := strings.Index(text, ".action{")
		if start < 0 {
			masked.WriteString(text)
			break
		}
		masked.WriteString(text[:start])
		end := start + len(".action{")
		var quote byte
		comment := false
		for ; end < len(text); end++ {
			c := text[end]
			if comment {
				if c == '*' && end+1 < len(text) && text[end+1] == '/' {
					comment = false
					end++
				}
				continue
			}
			if quote != 0 {
				if c == '\\' && quote != '`' {
					end++
					continue
				}
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == '/' && end+1 < len(text) && text[end+1] == '*' {
				comment = true
				end++
				continue
			}
			if c == '"' || c == '\'' || c == '`' {
				quote = c
				continue
			}
			if c == '}' {
				break
			}
		}
		if end >= len(text) {
			return nil
		}
		actions = append(actions, text[start:end+1])
		masked.WriteString(prefix + strings.Repeat("x", len(actions)))
		text = text[end+1:]
	}
	remaining := []byte(strings.TrimSuffix(strings.TrimPrefix(masked.String(), "{:"), "}"))
	var attrs [][]string
	isDoc := false
	for len(bytes.TrimSpace(remaining)) > 0 {
		valid, rest, attr, name, value := parse.TagAttr(remaining)
		if !valid || len(attr) == 0 {
			return nil
		}
		remaining = rest
		if string(name) == "type" && string(value) == "doc" {
			isDoc = true
		}
		val := string(value)
		// 长占位符优先替换，避免前缀相同的动作相互覆盖。
		for i := len(actions) - 1; i >= 0; i-- {
			val = strings.ReplaceAll(val, prefix+strings.Repeat("x", i+1), actions[i])
		}
		attrs = append(attrs, []string{string(name), val})
	}
	if !isDoc {
		return nil
	}
	return attrs
}
