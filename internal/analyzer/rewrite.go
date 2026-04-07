package analyzer

import (
	"go/token"
	"os"
	"sort"

	"golang.org/x/tools/go/analysis"
)

type fileEdit struct {
	start token.Pos
	end   token.Pos
	text  []byte
}

var editsByFile = map[string][]fileEdit{}

func addEdit(pass *analysis.Pass, pos, end token.Pos, text []byte) {
	file := pass.Fset.Position(pos).Filename
	editsByFile[file] = append(editsByFile[file], fileEdit{
		start: pos,
		end:   end,
		text:  text,
	})
}

// ApplyEdits writes all accumulated struct rewrites to their source files.
func ApplyEdits(fset *token.FileSet) error {
	for file, edits := range editsByFile {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		sort.Slice(edits, func(i, j int) bool {
			return edits[i].start > edits[j].start
		})

		data := content
		for _, e := range edits {
			start := fset.Position(e.start).Offset
			end := fset.Position(e.end).Offset
			result := make([]byte, 0, len(data)+(len(e.text)-(end-start)))
			result = append(result, data[:start]...)
			result = append(result, e.text...)
			result = append(result, data[end:]...)
			data = result
		}

		if err := os.WriteFile(file, data, 0644); err != nil {
			return err
		}
	}

	editsByFile = map[string][]fileEdit{}
	return nil
}
