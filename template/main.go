package main

import (
	"fmt"
	"os"

	"golang.org/x/mod/modfile"
)

// A Go modfile pretty printer.
//
// Not particularly useful, but a small enough example with a single dependency.

func main() {
	contents, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}

	mod, err := modfile.Parse("go.mod", contents, nil)
	if err != nil {
		panic(err)
	}

	pretty, err := mod.Format()
	if err != nil {
		panic(err)
	}

	fmt.Println(string(pretty))
}
