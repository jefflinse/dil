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

type Issue struct {
	PackageName string
	FileName    string
	Line        int
}

func (i *Issue) String() string {
	return fmt.Sprintf("External package %s used in file: %s, line: %d", i.PackageName, i.FileName, i.Line)
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

	ignoredFunctions := viper.GetStringSlice("ignore_functions")
	ignoredFuncsMap := make(map[string]struct{}, len(ignoredFunctions))
	for _, funcName := range ignoredFunctions {
		ignoredFuncsMap[funcName] = struct{}{}
	}

	var allIssues []Issue
	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			issues := inspectFile(file, allowedLibsMap, fileName, pkg, fs, ignoredFuncsMap)
			allIssues = append(allIssues, issues...)
		}
	}

	for _, issue := range allIssues {
		fmt.Println(issue.String())
	}
}

func inspectFile(file *ast.File, allowed map[string]struct{}, fileName string, pkg *ast.Package, fs *token.FileSet, ignoredFuncs map[string]struct{}) []Issue {
	var issues []Issue

	ast.Inspect(file, func(n ast.Node) bool {
		if funDecl, ok := n.(*ast.FuncDecl); ok {
			// If the function is in the ignore list, skip its inspection
			if _, isIgnored := ignoredFuncs[funDecl.Name.Name]; isIgnored {
				return false
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, isSelExpr := call.Fun.(*ast.SelectorExpr); isSelExpr {
				if x, isIdent := sel.X.(*ast.Ident); isIdent {
					// Ignore trivial packages from the standard library
					if _, isTrivial := allowed[x.Name]; isTrivial {
						return true
					}

					// Checking if the package being used is not the current package
					if !strings.EqualFold(x.Name, pkg.Name) {
						issues = append(issues, Issue{
							PackageName: x.Name,
							FileName:    fileName,
							Line:        fs.Position(call.Pos()).Line,
						})
					}
				}
			}
		}
		return true
	})

	return issues
}
