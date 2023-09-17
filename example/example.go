package example

import (
	"fmt"
	"io"
	"net/http"
)

// A package-level variable
var myPackageVar int

// MyFunction is an example function.
func MyFunction() {
	// A function-level variable
	var myFunctionVar int
	fmt.Fprintln(io.Discard, myFunctionVar)

	// A function-level constant
	const myFunctionConst = 1
	fmt.Fprintln(io.Discard, myFunctionConst)

	// Create an external dependency
	srv := &http.Server{}
	fmt.Fprintln(io.Discard, srv)
}
