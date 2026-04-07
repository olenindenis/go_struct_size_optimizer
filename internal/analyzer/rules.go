package analyzer

import (
	"go/ast"
	"go/token"
	"strings"
)

func buildIgnoreLines(fSet *token.FileSet, file *ast.File) map[int]bool {
	lines := map[int]bool{}
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.Contains(c.Text, "structalign:ignore") {
				lines[fSet.Position(c.Pos()).Line] = true
			}
		}
	}
	return lines
}

func isIgnored(fSet *token.FileSet, ignoreLines map[int]bool, node ast.Node) bool {
	line := fSet.Position(node.Pos()).Line
	return ignoreLines[line] || ignoreLines[line-1]
}

func hasTags(f *ast.Field) bool {
	return f.Tag != nil
}
