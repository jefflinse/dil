package main

import (
	"fmt"
	"net/http"
)

// Person struct definition
type Person struct {
	name string
}

// FetchSomething is a function that has a URL string parameter and
// makes an external package call using that parameter.
// The result is then assigned to a variable, which our linter should catch.
func FetchSomething(url string) {
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	// ... some more code here ...
}
