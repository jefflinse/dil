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

// Issue represents a single issue found by the linter.
type Issue struct {
	Package string `json:"package"`
	File    string `json:"file"`
	Line    int    `json:"line"`
}

// String returns a string representation of the issue.
func (i *Issue) String() string {
	return fmt.Sprintf("external package %s used at %s:%d", i.Package, i.File, i.Line)
}

// Runs the linter.
func runLinter(cmd *cobra.Command, args []string) {
	// arg[0] is the package path
	path := args[0]
	fs := token.NewFileSet()
	pkgs, err := parser.ParseDir(fs, path, nil, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("inspecting %d packages:", len(pkgs))
	for pkgName := range pkgs {
		log.Printf("  %s\n", pkgName)
	}

	allowedPackages := viper.GetStringSlice("allow_packages")
	allowedPkgsMap := make(map[string]struct{}, len(allowedPackages))
	for _, pkg := range allowedPackages {
		allowedPkgsMap[pkg] = struct{}{}
	}

	log.Printf("allowed packages: %v\n", allowedPackages)

	ignoredFunctions := viper.GetStringSlice("ignore_functions")
	ignoredFuncsMap := make(map[string]struct{}, len(ignoredFunctions))
	for _, funcName := range ignoredFunctions {
		ignoredFuncsMap[funcName] = struct{}{}
	}

	log.Printf("ignored functions: %v\n", ignoredFunctions)

	var allIssues []Issue
	for _, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			log.Printf("inspecting %s\n", fileName)
			issues := inspectFile(file, allowedPkgsMap, fileName, pkg, fs, ignoredFuncsMap)

			log.Printf("  found %d issues\n", len(issues))
			allIssues = append(allIssues, issues...)
		}
	}

	for _, issue := range allIssues {
		fmt.Println(issue.String())
	}
}

func inspectFile(file *ast.File, allowed map[string]struct{}, fileName string, pkg *ast.Package, fs *token.FileSet, ignoredFuncs map[string]struct{}) []Issue {
	var issues []Issue

	importMap := make(map[string]string)
	for _, imp := range file.Imports {
		fullPath := strings.Trim(imp.Path.Value, `"`) // Remove quotes around path
		// Get short name
		var shortName string
		if imp.Name != nil {
			shortName = imp.Name.Name
		} else {
			// Extract the last element from path if specific name isn't given
			splitPath := strings.Split(fullPath, "/")
			shortName = splitPath[len(splitPath)-1]
		}
		importMap[shortName] = fullPath
	}

	localVars := make([]string, 0)

	ast.Inspect(file, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhsExpr := range stmt.Lhs {
				if ident, isIdent := lhsExpr.(*ast.Ident); isIdent {
					localVars = append(localVars, ident.Name)
				}
			}
		case *ast.DeclStmt:
			if genDecl, isGenDecl := stmt.Decl.(*ast.GenDecl); isGenDecl && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					valueSpec := spec.(*ast.ValueSpec)
					for _, ident := range valueSpec.Names {
						localVars = append(localVars, ident.Name)
					}
				}
			}
		}

		if funDecl, ok := n.(*ast.FuncDecl); ok {
			// If the function is in the ignore list, skip its inspection
			if _, isIgnored := ignoredFuncs[funDecl.Name.Name]; isIgnored {
				return false
			}
		}
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, isSelExpr := call.Fun.(*ast.SelectorExpr); isSelExpr {
				if x, isIdent := sel.X.(*ast.Ident); isIdent {
					if isLocalVar, _ := inSlice(x.Name, localVars); isLocalVar {
						return true
					}

					// Get the full package path from importMap
					pkgPath, exists := importMap[x.Name]
					if !exists {
						pkgPath = x.Name // Fallback to the short name if not found in importMap
					}

					if _, isAllowed := allowed[pkgPath]; isAllowed {
						return true
					}

					// Checking if the package being used is not the current package
					if !strings.EqualFold(x.Name, pkg.Name) {
						issues = append(issues, Issue{
							Package: x.Name,
							File:    fileName,
							Line:    fs.Position(call.Pos()).Line,
						})
					}
				}
			}
		}
		return true
	})

	return issues
}

func inSlice(item string, slice []string) (bool, int) {
	for i, v := range slice {
		if v == item {
			return true, i
		}
	}
	return false, -1
}
