package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func runOnSource(t *testing.T, src string) []analysis.Diagnostic {
	t.Helper()

	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	if _, err = (&types.Config{}).Check("test", fSet, []*ast.File{f}, info); err != nil {
		t.Fatalf("typecheck: %v", err)
	}

	var diags []analysis.Diagnostic
	pass := &analysis.Pass{
		Fset:       fSet,
		Files:      []*ast.File{f},
		TypesInfo:  info,
		TypesSizes: types.SizesFor("gc", "amd64"),
		Report:     func(d analysis.Diagnostic) { diags = append(diags, d) },
	}
	if _, err = Analyzer.Run(pass); err != nil {
		t.Fatalf("run: %v", err)
	}
	return diags
}

// ── calcStructSize ────────────────────────────────────────────────────────────

func TestCalcStructSize(t *testing.T) {
	tests := []struct {
		name   string
		fields []fieldInfo
		want   int64
	}{
		{
			name:   "empty",
			fields: nil,
			want:   0,
		},
		{
			name:   "single bool",
			fields: []fieldInfo{{size: 1, align: 1}},
			want:   1,
		},
		{
			name:   "bool then int64 — internal padding",
			fields: []fieldInfo{{size: 1, align: 1}, {size: 8, align: 8}},
			want:   16, // 1 + 7_pad + 8
		},
		{
			name:   "int64 then bool — trailing padding",
			fields: []fieldInfo{{size: 8, align: 8}, {size: 1, align: 1}},
			want:   16, // 8 + 1 + 7_trail
		},
		{
			name: "suboptimal: bool int64 bool",
			fields: []fieldInfo{
				{size: 1, align: 1},
				{size: 8, align: 8},
				{size: 1, align: 1},
			},
			want: 24, // 1+7+8+1+7
		},
		{
			name: "optimal: int64 bool bool",
			fields: []fieldInfo{
				{size: 8, align: 8},
				{size: 1, align: 1},
				{size: 1, align: 1},
			},
			want: 16, // 8+1+1+6
		},
		{
			name: "int64 int32 bool",
			fields: []fieldInfo{
				{size: 8, align: 8},
				{size: 4, align: 4},
				{size: 1, align: 1},
			},
			want: 16, // 8+4+1+3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcStructSize(tt.fields); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// ── isHotField ────────────────────────────────────────────────────────────────

func TestIsHotField(t *testing.T) {
	hot := []string{"count", "Count", "requestCount", "flag", "IsFlag", "state", "CurrentState", "status", "StatusCode"}
	for _, name := range hot {
		if !isHotField(name) {
			t.Errorf("%q should be a hot field", name)
		}
	}

	cold := []string{"name", "age", "birthday", "value", "data", "id", "email"}
	for _, name := range cold {
		if isHotField(name) {
			t.Errorf("%q should be a cold field", name)
		}
	}
}

// ── optimizeFields ────────────────────────────────────────────────────────────

func TestOptimizeFields_HotFirst(t *testing.T) {
	fields := []fieldInfo{
		{name: "A", size: 1, align: 1},
		{name: "requestCount", size: 8, align: 8},
		{name: "B", size: 4, align: 4},
	}
	result := optimizeFields(fields)

	if len(result) != 3 {
		t.Fatalf("len=%d, want 3", len(result))
	}
	if result[0].name != "requestCount" {
		t.Errorf("first field: got %q, want requestCount", result[0].name)
	}
}

func TestOptimizeFields_DescendingAlignWithinGroup(t *testing.T) {
	fields := []fieldInfo{
		{name: "A", size: 1, align: 1},
		{name: "B", size: 4, align: 4},
		{name: "C", size: 8, align: 8},
	}
	result := optimizeFields(fields)

	wantAligns := []int64{8, 4, 1}
	for i, want := range wantAligns {
		if result[i].align != want {
			t.Errorf("result[%d].align=%d, want %d", i, result[i].align, want)
		}
	}
}

func TestOptimizeFields_OptimizationReducesSize(t *testing.T) {
	fields := []fieldInfo{
		{name: "A", size: 1, align: 1},
		{name: "B", size: 8, align: 8},
		{name: "C", size: 1, align: 1},
	}
	orig := calcStructSize(fields)
	opt := calcStructSize(optimizeFields(fields))

	if opt >= orig {
		t.Errorf("optimized size %d should be less than original %d", opt, orig)
	}
}

// ── buildIgnoreLines / isIgnored ──────────────────────────────────────────────

func TestBuildIgnoreLines(t *testing.T) {
	src := "package test\n//structalign:ignore\ntype T1 struct{}\n// normal comment\ntype T2 struct{}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	lines := buildIgnoreLines(fset, f)

	if !lines[2] {
		t.Error("line 2 (directive) should be marked")
	}
	if lines[4] {
		t.Error("line 4 (normal comment) should NOT be marked")
	}
}

func TestIsIgnored(t *testing.T) {
	src := "package test\n//structalign:ignore\ntype T1 struct{}\n\ntype T2 struct{}\n"
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var structs []*ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.StructType); ok {
			structs = append(structs, s)
		}
		return true
	})
	if len(structs) != 2 {
		t.Fatalf("expected 2 structs, got %d", len(structs))
	}

	ignoreLines := buildIgnoreLines(fSet, f)

	if !isIgnored(fSet, ignoreLines, structs[0]) {
		t.Error("T1 (preceded by directive) should be ignored")
	}
	if isIgnored(fSet, ignoreLines, structs[1]) {
		t.Error("T2 (no directive) should not be ignored")
	}
}

// ── hasTags ───────────────────────────────────────────────────────────────────

func TestHasTags(t *testing.T) {
	src := "package test\ntype T struct {\nA bool `json:\"a\"`\nB int64\n}"
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var st *ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.StructType); ok {
			st = s
			return false
		}
		return true
	})
	if st == nil {
		t.Fatal("struct not found")
	}

	if !hasTags(st.Fields.List[0]) {
		t.Error("field A should have tags")
	}
	if hasTags(st.Fields.List[1]) {
		t.Error("field B should not have tags")
	}
}

// ── buildFixedStruct ──────────────────────────────────────────────────────────

func TestBuildFixedStruct_ReordersFields(t *testing.T) {
	src := "package test\ntype T struct {\nA bool\nB int64\n}"
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var st *ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.StructType); ok {
			st = s
			return false
		}
		return true
	})

	optimized := []fieldInfo{
		{field: st.Fields.List[1]}, // B int64
		{field: st.Fields.List[0]}, // A bool
	}

	result, err := buildFixedStruct(optimized)
	if err != nil {
		t.Fatal(err)
	}

	got := string(result)
	aIdx := strings.Index(got, "A")
	bIdx := strings.Index(got, "B")
	if aIdx == -1 || bIdx == -1 {
		t.Fatalf("fields not found in output: %s", got)
	}
	if bIdx > aIdx {
		t.Errorf("expected B before A:\n%s", got)
	}
}

// ── buildDiff ─────────────────────────────────────────────────────────────────

func TestBuildDiff_Markers(t *testing.T) {
	old := "struct {\n\tA bool\n\tB int64\n}"
	new_ := "struct {\n\tB int64\n\tA bool\n}"

	got := buildDiff(old, new_)

	if !strings.Contains(got, " - ") {
		t.Error("expected ' - ' marker for removed lines")
	}
	if !strings.Contains(got, " + ") {
		t.Error("expected ' + ' marker for added lines")
	}
	if !strings.Contains(got, "─") {
		t.Error("expected '─' divider")
	}
}

func TestBuildDiff_IdenticalInputs(t *testing.T) {
	text := "struct {\n\tA bool\n}"
	got := buildDiff(text, text)

	if strings.Contains(got, " - ") || strings.Contains(got, " + ") {
		t.Error("identical inputs should produce no -/+ markers")
	}
}

func TestBuildDiff_LineNumbers(t *testing.T) {
	old := "struct {\n\tA bool\n\tB int64\n}"
	new_ := "struct {\n\tB int64\n\tA bool\n}"
	got := buildDiff(old, new_)

	if !strings.Contains(got, "1") {
		t.Error("expected line number 1 in output")
	}
}

// ── run (integration) ─────────────────────────────────────────────────────────

func TestRun_ReportsOptimization(t *testing.T) {
	src := "package test\ntype T struct {\nA bool\nB int64\nC bool\n}"
	diags := runOnSource(t, src)

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0].Message, "24 -> 16") {
		t.Errorf("unexpected message: %s", diags[0].Message)
	}
	if len(diags[0].SuggestedFixes) == 0 {
		t.Error("expected SuggestedFixes to be present")
	}
}

func TestRun_NoReportWhenAlreadyOptimal(t *testing.T) {
	src := "package test\ntype T struct {\nB int64\nA bool\nC bool\n}"
	diags := runOnSource(t, src)

	if len(diags) != 0 {
		t.Errorf("expected no diagnostics, got %d", len(diags))
	}
}

func TestRun_SkipsTaggedStruct(t *testing.T) {
	src := "package test\ntype T struct {\nA bool `json:\"a\"`\nB int64\nC bool\n}"
	diags := runOnSource(t, src)

	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for struct with tags, got %d", len(diags))
	}
}

func TestRun_IgnoreDirective(t *testing.T) {
	src := "package test\n//structalign:ignore\ntype T struct {\nA bool\nB int64\nC bool\n}"
	diags := runOnSource(t, src)

	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for ignored struct, got %d", len(diags))
	}
}

func TestRun_EmptyStruct(t *testing.T) {
	src := "package test\ntype T struct{}"
	diags := runOnSource(t, src)

	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for empty struct, got %d", len(diags))
	}
}

func TestRun_MultipleStructs_OnlyBadReported(t *testing.T) {
	src := `package test
type Good struct {
	B int64
	A bool
	C bool
}
type Bad struct {
	A bool
	B int64
	C bool
}
`
	diags := runOnSource(t, src)

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic (for Bad), got %d", len(diags))
	}
}

func TestRun_NilTypesInfo(t *testing.T) {
	pass := &analysis.Pass{
		Fset:       token.NewFileSet(),
		TypesInfo:  nil,
		TypesSizes: types.SizesFor("gc", "amd64"),
		Report:     func(d analysis.Diagnostic) {},
	}
	_, err := Analyzer.Run(pass)
	if err != nil {
		t.Errorf("expected no error with nil TypesInfo, got: %v", err)
	}
}

// ── applyEdits ────────────────────────────────────────────────────────────────

func TestApplyEdits_ReplacesStructInFile(t *testing.T) {
	src := "package main\n\ntype T struct {\n\tA bool\n\tB int64\n}\n"

	path := filepath.Join(t.TempDir(), "test.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, path, src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var st *ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.StructType); ok {
			st = s
			return false
		}
		return true
	})
	if st == nil {
		t.Fatal("struct not found")
	}

	editsByFile = map[string][]fileEdit{} // clean state

	pass := &analysis.Pass{Fset: fSet}
	addEdit(pass, st.Pos(), st.End(), []byte("struct {\n\tB int64\n\tA bool\n}"))

	if err := ApplyEdits(fSet); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(got)

	if strings.Contains(result, "\tA bool\n\tB int64") {
		t.Error("original field order should not appear after edit")
	}
	if !strings.Contains(result, "\tB int64\n\tA bool") {
		t.Errorf("expected reordered fields:\n%s", result)
	}
}

func TestApplyEdits_ClearsStateAfterApply(t *testing.T) {
	src := "package main\ntype T struct {\n\tA bool\n}\n"

	path := filepath.Join(t.TempDir(), "test.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, path, src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var st *ast.StructType
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.StructType); ok {
			st = s
		}
		return true
	})

	editsByFile = map[string][]fileEdit{}
	pass := &analysis.Pass{Fset: fSet}
	addEdit(pass, st.Pos(), st.End(), []byte("struct {\n\tA bool\n}"))

	if err := ApplyEdits(fSet); err != nil {
		t.Fatal(err)
	}

	if len(editsByFile) != 0 {
		t.Error("editsByFile should be empty after applyEdits")
	}
}
