package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	var cmd = &cobra.Command{
		Use:   "linter [path]",
		Short: "Linter to detect external package usages",
		Run:   runLinter,
		Args:  cobra.ExactArgs(1),
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runLinter(cmd *cobra.Command, args []string) {
	path := args[0]
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, path, nil, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, isSelExpr := call.Fun.(*ast.SelectorExpr); isSelExpr {
						if x, isIdent := sel.X.(*ast.Ident); isIdent {
							// Checking if the package being used is not the current package
							if !strings.EqualFold(x.Name, pkg.Name) {
								fmt.Printf("External package %s used in file: %s, line: %d\n", x.Name, fileName, fs.Position(call.Pos()).Line)
							}
						}
					}
				}
				return true
			})
		}
	}
}
