package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "linter [path]",
		Short: "Lint Go code for inline external object creation",
		Args:  cobra.ExactArgs(1),
		Run:   runLinter,
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runLinter(cmd *cobra.Command, args []string) {
	pkgPath := args[0]
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, pkgPath, nil, 0)
	if err != nil {
		fmt.Printf("Error parsing package: %s\n", err)
		return
	}

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.CallExpr:
					// Checking if this call is a creation of an external object
					if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := sel.X.(*ast.Ident); ok {
							if ident.Obj == nil && !isBuiltin(ident.Name) {
								fmt.Printf("File: %s, Line: %d, External object %s created\n",
									fileName, fs.Position(x.Pos()).Line, ident.Name)
							}
						}
					}
				}
				return true
			})
		}
	}
}

func isBuiltin(name string) bool {
	// For simplicity, just checking against Go built-in packages.
	// This list can be extended further based on requirements.
	builtins := []string{"fmt", "os", "strings", "bytes", "errors", "time", "math"}
	for _, b := range builtins {
		if b == name {
			return true
		}
	}
	return false
}
