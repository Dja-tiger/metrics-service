package main

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

var analyzer = &analysis.Analyzer{
	Name: "noexit",
	Doc:  "reports builtin panic and calls to log.Fatal, log.Fatalf, log.Fatalln or os.Exit outside main.main",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Walk(visitor{pass: pass}, file)
	}
	return nil, nil
}

type visitor struct {
	pass   *analysis.Pass
	inMain bool
}

func (v visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		v.inMain = v.pass.Pkg.Name() == "main" && n.Name.Name == "main" && n.Recv == nil
	case *ast.FuncLit:
		// A closure has its own function scope, even when declared in main.
		v.inMain = false
	case *ast.CallExpr:
		var object types.Object
		switch fun := ast.Unparen(n.Fun).(type) {
		case *ast.Ident:
			object = v.pass.TypesInfo.Uses[fun]
		case *ast.SelectorExpr:
			object = v.pass.TypesInfo.Uses[fun.Sel]
		}
		if builtin, ok := object.(*types.Builtin); ok && builtin.Name() == "panic" {
			v.pass.Reportf(n.Pos(), "builtin panic is forbidden")
		}
		fn, ok := object.(*types.Func)
		if !ok || fn.Pkg() == nil || v.inMain {
			break
		}
		// Methods such as testing.T.Fatal are not package-level log functions.
		if fn.Type().(*types.Signature).Recv() != nil {
			break
		}
		path, name := fn.Pkg().Path(), fn.Name()
		if path == "os" && name == "Exit" || path == "log" && (name == "Fatal" || name == "Fatalf" || name == "Fatalln") {
			v.pass.Reportf(n.Pos(), "%s.%s is forbidden outside main.main", path, name)
		}
	}
	return v
}
