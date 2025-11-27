package main

import (
	"fmt"
	"github.com/alecthomas/kong"
	"reflect"
)

var CLI struct {
	Rm struct {
		Force     bool `help:"Force removal."`
		Recursive bool `help:"Recursively remove files."`

		Paths []string `arg:"" name:"path" help:"Paths to remove." type:"path"`
	} `cmd:"" help:"Remove files."`

	Ls struct {
		Paths []string `arg:"" optional:"" name:"path" help:"Paths to list." type:"path"`
	} `cmd:"" help:"List paths."`
}

func main() {
	ctx := kong.Parse(&CLI)

	fmt.Println(reflect.TypeOf(ctx))

	switch ctx.Command() {
	case "rm <path>":
	case "ls":
	default:
		panic(ctx.Command())
	}
}
