package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	var cmd = &cobra.Command{
		Use:   "linter [path]",
		Short: "Linter to detect significant external package usages",
		Run:   runLinter,
		Args:  cobra.ExactArgs(1),
	}

	setupConfig()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func setupConfig() {
	viper.SetConfigName(".didetect")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
}

func runLinter(cmd *cobra.Command, args []string) {
	path := args[0]
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, path, nil, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}

	allowedStdLibs := viper.GetStringSlice("allowed_std_libs")
	allowedLibsMap := make(map[string]struct{}, len(allowedStdLibs))
	for _, lib := range allowedStdLibs {
		allowedLibsMap[lib] = struct{}{}
	}

	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if sel, isSelExpr := call.Fun.(*ast.SelectorExpr); isSelExpr {
						if x, isIdent := sel.X.(*ast.Ident); isIdent {
							// Ignore trivial packages from the standard library
							if _, isTrivial := allowedLibsMap[x.Name]; isTrivial {
								return true
							}

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
