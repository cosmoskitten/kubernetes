/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// list all unit and ginkgo test names that will be run
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	dumpTree            = flag.Bool("dump", false, "print AST")
	dumpJson            = flag.Bool("json", false, "output test list as JSON")
	warn                = flag.Bool("warn", false, "print warnings")
	writeConformanceDoc = flag.Bool("conformance", false, "write a conformance document")
)

type Test struct {
	Loc      string
	Name     string
	TestName string
	Comment  string
}

type ConformanceData struct {
	// A URL to the line of code in the kube src repo for the test
	URL string
	// Extracted from the "Testname:" comment before the test
	TestName string
	// Extracted from the "Description:" comment before the test
	Description string
}

func convertToConformanceData(test *Test) ConformanceData {
	baseURL := "https://github.com/kubernetes/kubernetes/tree/master/"

	cd := ConformanceData{}

	parts := strings.SplitN(test.Loc, ":", 3)
	if len(parts) > 1 {
		cd.URL = baseURL + parts[0] + "#L" + parts[1]
	} else {
		cd.URL = baseURL + test.Loc
	}

	lines := strings.Split(test.Comment, "\n")
	cd.Description = ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Testname:") {
			line = strings.TrimSpace(line[9:])
			cd.TestName = line
			continue
		}
		if strings.HasPrefix(line, "Description:") {
			line = strings.TrimSpace(line[12:])
		}
		cd.Description += line + "\n"
	}

	if cd.TestName == "" {
		i := strings.Index(test.TestName, "[")
		if i > 0 {
			cd.TestName = strings.TrimSpace(test.TestName[:i])
		} else {
			cd.TestName = test.TestName
		}
	}

	return cd
}

// collect extracts test metadata from a file.
// If src is nil, it reads filename for the code, otherwise it
// uses src (which may be a string, byte[], or io.Reader).
func collect(filename string, src interface{}) []Test {
	// Create the AST by parsing src.
	fset := token.NewFileSet() // positions are relative to fset
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	//create comment map
	cmap := ast.NewCommentMap(fset, f, f.Comments)

	if *dumpTree {
		ast.Print(fset, f)
	}

	tests := make([]Test, 0)

	ast.Walk(makeWalker("[k8s.io]", fset, &tests, cmap), f)

	// Unit tests are much simpler to enumerate!
	if strings.HasSuffix(filename, "_test.go") {
		packageName := f.Name.Name
		dirName, _ := filepath.Split(filename)
		if filepath.Base(dirName) != packageName && *warn {
			log.Printf("Warning: strange path/package mismatch %s %s\n", filename, packageName)
		}
		testPath := "k8s.io/kubernetes/" + dirName[:len(dirName)-1]
		for _, decl := range f.Decls {
			funcdecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := funcdecl.Name.Name
			if strings.HasPrefix(name, "Test") {
				tests = append(tests, Test{fset.Position(funcdecl.Pos()).String(), testPath, name, funcdecl.Doc.Text()})
			}
		}
	}

	return tests
}

// funcName converts a selectorExpr with two idents into a string,
// x.y -> "x.y"
func funcName(n ast.Expr) string {
	if sel, ok := n.(*ast.SelectorExpr); ok {
		if x, ok := sel.X.(*ast.Ident); ok {
			return x.String() + "." + sel.Sel.String()
		}
	}
	return ""
}

// isSprintf returns whether the given node is a call to fmt.Sprintf
func isSprintf(n ast.Expr) bool {
	call, ok := n.(*ast.CallExpr)
	return ok && funcName(call.Fun) == "fmt.Sprintf" && len(call.Args) != 0
}

type walker struct {
	path  string
	fset  *token.FileSet
	tests *[]Test
	vals  map[string]string
	CMap  ast.CommentMap
}

func makeWalker(path string, fset *token.FileSet, tests *[]Test, cmap ast.CommentMap) *walker {
	return &walker{path, fset, tests, make(map[string]string), cmap}
}

// clone creates a new walker with the given string extending the path.
func (w *walker) clone(ext string) *walker {
	return &walker{w.path + " " + ext, w.fset, w.tests, w.vals, w.CMap}
}

// firstArg attempts to statically determine the value of the first
// argument. It only handles strings, and converts any unknown values
// (fmt.Sprintf interpolations) into *.
func (w *walker) firstArg(n *ast.CallExpr) string {
	if len(n.Args) == 0 {
		return ""
	}
	var lit *ast.BasicLit
	if isSprintf(n.Args[0]) {
		return w.firstArg(n.Args[0].(*ast.CallExpr))
	}
	lit, ok := n.Args[0].(*ast.BasicLit)
	if ok && lit.Kind == token.STRING {
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			panic(err)
		}
		if strings.Contains(v, "%") {
			v = strings.Replace(v, "%d", "*", -1)
			v = strings.Replace(v, "%v", "*", -1)
			v = strings.Replace(v, "%s", "*", -1)
		}
		return v
	}
	if ident, ok := n.Args[0].(*ast.Ident); ok {
		if val, ok := w.vals[ident.String()]; ok {
			return val
		}
	}
	if *warn {
		log.Printf("Warning: dynamic arg value: %v\n", w.fset.Position(n.Args[0].Pos()))
	}
	return "*"
}

// describeName returns the first argument of a function if it's
// a Ginkgo-relevant function (Describe/KubeDescribe/Context),
// and the empty string otherwise.
func (w *walker) describeName(n *ast.CallExpr) string {
	switch x := n.Fun.(type) {
	case *ast.SelectorExpr:
		if x.Sel.Name != "KubeDescribe" {
			return ""
		}
	case *ast.Ident:
		if x.Name != "Describe" && x.Name != "Context" {
			return ""
		}
	default:
		return ""
	}
	return w.firstArg(n)
}

// itName returns the first argument if it's a call to It(), else "".
func (w *walker) itName(n *ast.CallExpr) string {
	if fun, ok := n.Fun.(*ast.Ident); ok && fun.Name == "It" {
		return w.firstArg(n)
	}
	return ""
}

// Visit walks the AST, following Ginkgo context and collecting tests.
// See the documentation for ast.Walk for more details.
func (w *walker) Visit(n ast.Node) ast.Visitor {
	switch x := n.(type) {
	case *ast.CallExpr:
		name := w.describeName(x)
		if name != "" && len(x.Args) >= 2 {
			// If calling (Kube)Describe/Context, make a new
			// walker to recurse with the description added.
			return w.clone(name)
		}
		name = w.itName(x)
		comment := ""
		for _, comm := range w.CMap.Comments() {
			//make sure to pick up the comment that is above the It block
			//comment may a line feed stripped and hence the exn position of comment
			//could typically be 3 character behid the start of It block.
			if x.Pos() > comm.End() && x.Pos()-comm.End() <= 3 {
				comment = comm.Text()
			}
		}

		if name != "" {
			// We've found an It() call, the full test name
			// can be determined now.
			if w.path == "[k8s.io]" && *warn {
				log.Printf("It without matching Describe: %s\n", w.fset.Position(n.Pos()))
			}
			*w.tests = append(*w.tests, Test{w.fset.Position(n.Pos()).String(), w.path, name, comment})
			return nil // Stop walking
		}
	case *ast.AssignStmt:
		// Attempt to track literals that might be used as
		// arguments. This analysis is very unsound, and ignores
		// both scope and program flow, but is sufficient for
		// our minor use case.
		ident, ok := x.Lhs[0].(*ast.Ident)
		if ok {
			if isSprintf(x.Rhs[0]) {
				// x := fmt.Sprintf("something", args)
				w.vals[ident.String()] = w.firstArg(x.Rhs[0].(*ast.CallExpr))
			}
			if lit, ok := x.Rhs[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				// x := "a literal string"
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					panic(err)
				}
				w.vals[ident.String()] = v
			}
		}
	}
	return w // Continue walking
}

type testList struct {
	tests []Test
}

// handlePath walks the filesystem recursively, collecting tests
// from files with paths *e2e*.go and *_test.go, ignoring third_party
// and staging directories.
func (t *testList) handlePath(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if strings.Contains(path, "third_party") ||
		strings.Contains(path, "staging") ||
		strings.Contains(path, "_output") {
		return filepath.SkipDir
	}
	if strings.HasSuffix(path, ".go") && strings.Contains(path, "e2e") ||
		strings.HasSuffix(path, "_test.go") {
		tests := collect(path, nil)
		t.tests = append(t.tests, tests...)
	}
	return nil
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = append(args, ".")
	}
	tests := testList{}
	for _, arg := range args {
		err := filepath.Walk(arg, tests.handlePath)
		if err != nil {
			log.Fatalf("Error walking: %v", err)
		}
	}

	if *dumpJson {
		json, err := json.Marshal(tests.tests)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(json))
	} else if *writeConformanceDoc {
		file, err := os.Create("Conformance.md")
		if err != nil {
			log.Fatalf("Error creating file Conformance.md: %v", err)
		}
		defer file.Close()

		// Note: this assumes that you're running from the root of the kube src repo
		header, err := ioutil.ReadFile("test/list/cf_header.md")
		if err == nil {
			file.Write([]byte(fmt.Sprintf("%s\n\n", header)))
		}

		for _, t := range tests.tests {
			if matched, err := regexp.MatchString("\\[Conformance\\]", t.TestName); err == nil && matched {
				cd := convertToConformanceData(&t)
				hdr := fmt.Sprintf("## [%s](%s)\n\n", cd.TestName, cd.URL)
				file.Write([]byte(hdr))
				file.Write([]byte(cd.Description + "\n\n"))
			}
		}
	} else {
		for _, t := range tests.tests {
			fmt.Println(t)
		}
	}
}
