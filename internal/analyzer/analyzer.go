// Package analyzer provides Go source file parsing and analysis.
package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// FileInfo contains parsed information about a Go source file.
type FileInfo struct {
	Path      string
	Package   string
	Imports   []string
	Functions []FuncInfo
	Types     []TypeInfo
	Vars      []VarInfo
	Lines     int
}

// FuncInfo describes a function or method.
type FuncInfo struct {
	Name     string
	Receiver string // empty for functions, type name for methods
	Line     int
	EndLine  int
}

// TypeInfo describes a type declaration.
type TypeInfo struct {
	Name    string
	Kind    string // struct, interface, alias
	Line    int
	EndLine int
}

// VarInfo describes a variable or constant declaration.
type VarInfo struct {
	Name  string
	IsVar bool // true for var, false for const
	Line  int
}

// CountLines returns the number of lines in the content.
// A trailing newline adds a line (standard wc -l behavior + 1 for content).
func CountLines(content string) int {
	if content == "" {
		return 1
	}
	// Count newlines and add 1 for the first line
	return strings.Count(content, "\n") + 1
}

// ParseGoFile parses a Go source file and returns information about its contents.
func ParseGoFile(path string) (*FileInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	info := &FileInfo{
		Path:    path,
		Package: file.Name.Name,
		Lines:   CountLines(string(content)),
	}

	// Extract imports
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		info.Imports = append(info.Imports, path)
	}

	// Walk the AST to extract declarations
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			fn := FuncInfo{
				Name:    decl.Name.Name,
				Line:    fset.Position(decl.Pos()).Line,
				EndLine: fset.Position(decl.End()).Line,
			}
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				fn.Receiver = exprToString(decl.Recv.List[0].Type)
			}
			info.Functions = append(info.Functions, fn)

		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					ti := TypeInfo{
						Name:    s.Name.Name,
						Line:    fset.Position(s.Pos()).Line,
						EndLine: fset.Position(s.End()).Line,
					}
					switch s.Type.(type) {
					case *ast.StructType:
						ti.Kind = "struct"
					case *ast.InterfaceType:
						ti.Kind = "interface"
					default:
						ti.Kind = "alias"
					}
					info.Types = append(info.Types, ti)

				case *ast.ValueSpec:
					for _, name := range s.Names {
						info.Vars = append(info.Vars, VarInfo{
							Name:  name.Name,
							IsVar: decl.Tok == token.VAR,
							Line:  fset.Position(s.Pos()).Line,
						})
					}
				}
			}
		}
		return true
	})

	return info, nil
}

// exprToString converts a type expression to a string representation.
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	default:
		return ""
	}
}
