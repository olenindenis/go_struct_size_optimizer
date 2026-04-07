package analyzer

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

func buildFixedStruct(optimized []fieldInfo) ([]byte, error) {
	newFields := make([]*ast.Field, 0, len(optimized))
	for _, opt := range optimized {
		newFields = append(newFields, &ast.Field{
			Names: opt.field.Names,
			Type:  opt.field.Type,
			Tag:   opt.field.Tag,
		})
	}

	newStruct := &ast.StructType{
		Fields: &ast.FieldList{
			List: newFields,
		},
	}

	var buf bytes.Buffer
	err := format.Node(&buf, token.NewFileSet(), newStruct)
	return buf.Bytes(), err
}

func renderNode(pass *analysis.Pass, node any) ([]byte, error) {
	var buf bytes.Buffer
	err := format.Node(&buf, pass.Fset, node)
	return buf.Bytes(), err
}
