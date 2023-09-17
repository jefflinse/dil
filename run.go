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

	packageResults := map[string]PackageResult{}
	for _, pkg := range pkgs {
		log.Printf("inspecting package %s\n", pkg.Name)
		packageResults[pkg.Name] = PackageResult{
			PackageVars: map[string]struct{}{},
			FileResults: map[string]*FileResult{},
		}

		// get package variables declared in the package
		for _, file := range pkg.Files {
			packageVars := getPackageVariablesDeclaredInFile(file)
			for k := range packageVars {
				packageResults[pkg.Name].PackageVars[k] = struct{}{}
			}
		}

		allowedPackagePkgs := map[string]struct{}{}
		for k := range allowedPkgsMap {
			allowedPackagePkgs[k] = struct{}{}
		}
		for k := range packageResults[pkg.Name].PackageVars {
			allowedPackagePkgs[k] = struct{}{}
		}

		for fileName, file := range pkg.Files {
			result := inspectFile(file, allowedPackagePkgs, fileName, pkg, fs, ignoredFuncsMap)
			log.Printf("  found %d issues\n", len(result.Issues))
			if _, ok := packageResults[pkg.Name]; !ok {
				packageResults[pkg.Name] = PackageResult{
					FileResults: map[string]*FileResult{},
					PackageVars: map[string]struct{}{},
				}
			}

			packageResults[pkg.Name].FileResults[fileName] = result
		}
	}

	for pkgName, pkgResults := range packageResults {
		fmt.Printf("package %s\n", pkgName)
		for fileName, fileResult := range pkgResults.FileResults {
			fmt.Printf("  %s (%d issues)\n", fileName, len(fileResult.Issues))
			fmt.Printf("  imports: %v\n", fileResult.Imports)
			fmt.Printf("  local vars: %v\n", fileResult.LocalVars)
			for _, issue := range fileResult.Issues {
				fmt.Printf("    %s\n", issue.String())
			}
		}
	}
}

type PackageResult struct {
	PackageVars map[string]struct{}
	FileResults map[string]*FileResult
}

type FileResult struct {
	Imports   map[string]string
	LocalVars map[string]struct{}
	Issues    []Issue
}

func getPackageVariablesDeclaredInFile(file *ast.File) map[string]struct{} {
	packageVariables := map[string]struct{}{}

	ast.Inspect(file, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.GenDecl:
			if stmt.Tok != token.VAR {
				return false
			}

			log.Println("GENERIC DECLARATION")

			for _, spec := range stmt.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					log.Printf("unexpected spec type: %T\n", spec)
				}

				// valueSpec.Names contains the names of the variables declared in this spec
				for _, name := range valueSpec.Names {
					packageVariables[name.Name] = struct{}{}
					log.Printf("found package variable: %v\n", name.Name)
				}
			}
		}

		return false
	})

	return packageVariables
}

func inspectFile(file *ast.File, allowed map[string]struct{}, fileName string, pkg *ast.Package, fs *token.FileSet, ignoredFuncs map[string]struct{}) *FileResult {
	result := &FileResult{
		Imports:   map[string]string{},
		LocalVars: map[string]struct{}{},
		Issues:    []Issue{},
	}

	// build map of import names to full paths
	for _, imp := range file.Imports {
		fullPath := strings.Trim(imp.Path.Value, `"`)
		var shortName string
		if imp.Name != nil {
			shortName = imp.Name.Name
		} else {
			// TODO: this is a hack, should be able to get the package name from the import.
			// This probably doesn't handle /v2 or other suffixes.
			splitPath := strings.Split(fullPath, "/")
			shortName = splitPath[len(splitPath)-1]
		}

		result.Imports[shortName] = fullPath
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			result = handleAssignStmt(result, stmt, fs)
		case *ast.CallExpr:
			log.Printf("CALL %s, (line %d)", stmt.Fun, fs.Position(stmt.Pos()).Line)
			switch ct := stmt.Fun.(type) {
			case *ast.Ident:
				log.Printf("  IDENT: %+v\n", ct)
				log.Printf("  IDENT NAME: %+v\n", ct.Name)
			case *ast.SelectorExpr:
				log.Printf("  SELECTOR: %+v\n", ct)
				log.Printf("    EXPRESSION: %+v\n", ct.X)
				log.Printf("    FIELD SELCTOR: %+v\n", ct.Sel)
				switch x := ct.X.(type) {
				case *ast.Ident:
					log.Printf("found function call: %v\n", x.Name)

					// the first part of the selector might be a package name
					// check if it's a local variable or a package name

					if _, isLocalVar := result.LocalVars[x.Name]; isLocalVar {
						return true
					}

					// Get the full package path from importMap
					pkgPath, exists := result.Imports[x.Name]
					if !exists {
						pkgPath = x.Name // Fallback to the short name if not found in importMap
					}

					if _, isAllowed := allowed[pkgPath]; isAllowed {
						return true
					}

					// Checking if the package being used is not the current package
					if !strings.EqualFold(x.Name, pkg.Name) {
						result.Issues = append(result.Issues, Issue{
							Package: x.Name,
							File:    fileName,
							Line:    fs.Position(stmt.Pos()).Line,
						})
					}
				case *ast.SelectorExpr:
					log.Printf("found function call?: %v\n", x.Sel.Name)
				default:
					log.Printf("unexpected selector expression type: %T\n", x)
				}
			default:
				log.Printf("unexpected function expression type: %T\n", ct)

			}

		case *ast.DeclStmt:
			log.Println("DECLARATION")
			switch decl := stmt.Decl.(type) {
			case *ast.GenDecl:
				log.Println("  GENERIC DECLARATION:")
				log.Printf("    TOK: %+v\n", decl.Tok)
				log.Printf("    SPECS: %+v\n", decl.Specs)
			case *ast.FuncDecl:
				log.Println("  FUNCTION DECLARATION:")
				log.Printf("    NAME: %+v\n", decl.Name)
				log.Printf("    RECV: %+v\n", decl.Recv)
				log.Printf("    TYPE: %+v\n", decl.Type)
				log.Printf("    BODY: %+v\n", decl.Body)
			default:
				log.Printf("  unexpected declaration type: %T\n", decl)
			}

			if genDecl, isGenDecl := stmt.Decl.(*ast.GenDecl); isGenDecl && genDecl.Tok == token.VAR {
				for _, spec := range genDecl.Specs {
					valueSpec := spec.(*ast.ValueSpec)
					for _, ident := range valueSpec.Names {
						result.LocalVars[ident.Name] = struct{}{}
					}
				}
			}

		case *ast.ValueSpec:
			log.Println("VALUE SPEC")
			log.Printf("  NAMES: %+v\n", stmt.Names)
			log.Printf("  VALUES: %+v\n", stmt.Values)
			log.Printf("  TYPE: %+v\n", stmt.Type)

		case *ast.FuncDecl:
			// if the function is in the ignore list, skip its inspection
			if _, isIgnored := ignoredFuncs[stmt.Name.Name]; isIgnored {
				return false
			}

			log.Printf("FUNCTION DECLARATION: %v\n", stmt.Name.Name)

		case *ast.GenDecl:
			if stmt.Tok != token.VAR {
				return false
			}

			log.Println("GENERIC DECLARATION")
			switch spec := stmt.Specs[0].(type) {
			case *ast.ValueSpec:
				log.Printf("  VALUE SPEC: %+v\n", spec)
				log.Printf("    NAMES: %+v\n", spec.Names)
				log.Printf("    VALUES: %+v\n", spec.Values)
				log.Printf("    TYPE: %+v\n", spec.Type)
			case *ast.TypeSpec:
				log.Printf("  TYPE SPEC: %+v\n", spec)
				log.Printf("    NAME: %+v\n", spec.Name)
				log.Printf("    TYPE: %+v\n", spec.Type)
			default:
				log.Printf("  unexpected spec type: %T\n", spec)
			}
		}

		return true
	})

	return result
}

func handleAssignStmt(result *FileResult, stmt *ast.AssignStmt, fs *token.FileSet) *FileResult {
	log.Printf("ASSIGNMENT %v", stmt)
	log.Printf("  LHS: %+v\n", stmt.Lhs)
	switch rhs := stmt.Rhs[0].(type) {
	case *ast.CallExpr:
		log.Printf("    CALL %s, (line %d)", rhs.Fun, fs.Position(rhs.Pos()).Line)
	case *ast.Ident:
		log.Printf("    IDENT %s, (line %d)", rhs.Name, fs.Position(rhs.Pos()).Line)
	case *ast.SelectorExpr:
		log.Printf("    SELECTOR %s, (line %d)", rhs.Sel, fs.Position(rhs.Pos()).Line)
	case *ast.UnaryExpr:
		log.Printf("    UNARY %s, (line %d)", rhs.Op, fs.Position(rhs.Pos()).Line)
		log.Printf("      X: %+v\n", rhs.X)
		switch x := rhs.X.(type) {
		case *ast.Ident:
			log.Printf("        IDENT: %+v\n", x)
			log.Printf("        IDENT NAME: %+v\n", x.Name)
		case *ast.SelectorExpr:
			log.Printf("        SELECTOR: %+v\n", x)
			log.Printf("          EXPRESSION: %+v\n", x.X)
			log.Printf("          FIELD SELCTOR: %+v\n", x.Sel)
		case *ast.CompositeLit:
			log.Printf("        COMPOSITE LITERAL %+v\n", x)
			switch elt := x.Type.(type) {
			case *ast.Ident:
				log.Printf("          IDENT: %+v\n", elt)
				log.Printf("          IDENT NAME: %+v\n", elt.Name)
			case *ast.SelectorExpr:
				log.Printf("          SELECTOR: %+v\n", elt)
				log.Printf("            EXPRESSION: %+v\n", elt.X)
				log.Printf("            FIELD SELCTOR: %+v\n", elt.Sel)
				// the expression here is a candidate for an external package reference
				log.Printf("--- EXTERNAL PKG ---> %+v\n", elt.X)
			default:
				log.Printf("          unexpected composite literal type: %T\n", elt)
			}
		default:
			log.Printf("        unexpected unary expression type: %T\n", x)
		}
	case *ast.CompositeLit:
		log.Printf("    COMPOSITE LITERAL %+v\n", rhs)
		// a composite literal can contain something referening an external package
		log.Println("DANGER WILL ROBINSON")
	case *ast.BasicLit:
		log.Printf("    BASIC LITERAL %+v\n", rhs)
	case *ast.BinaryExpr:
		log.Printf("    BINARY EXPR %+v\n", rhs)
	case *ast.IndexExpr:
		log.Printf("    INDEX EXPR %+v\n", rhs)
	case *ast.StarExpr:
		log.Printf("    STAR EXPR %+v\n", rhs)
	case *ast.SliceExpr:
		log.Printf("    SLICE EXPR %+v\n", rhs)
	case *ast.KeyValueExpr:
		log.Printf("    KEY VALUE EXPR %+v\n", rhs)
	case *ast.ParenExpr:
		log.Printf("    PAREN EXPR %+v\n", rhs)
	case *ast.TypeAssertExpr:
		log.Printf("    TYPE ASSERT EXPR %+v\n", rhs)
	case *ast.FuncLit:
		log.Printf("    FUNC LIT %+v\n", rhs)
	case *ast.Ellipsis:
		log.Printf("    ELLIPSIS %+v\n", rhs)
	case *ast.FuncType:
		log.Printf("    FUNC TYPE %+v\n", rhs)
	case *ast.InterfaceType:
		log.Printf("    INTERFACE TYPE %+v\n", rhs)
	case *ast.MapType:
		log.Printf("    MAP TYPE %+v\n", rhs)
	case *ast.ChanType:
		log.Printf("    CHAN TYPE %+v\n", rhs)
	case *ast.ArrayType:
		log.Printf("    ARRAY TYPE %+v\n", rhs)
	case *ast.StructType:
		log.Printf("    STRUCT TYPE %+v\n", rhs)
	default:
		log.Printf("    unexpected rhs type: %T\n", rhs)
	}
	for _, lhsExpr := range stmt.Lhs {
		if ident, isIdent := lhsExpr.(*ast.Ident); isIdent {
			result.LocalVars[ident.Name] = struct{}{}
		}
	}

	return result
}
