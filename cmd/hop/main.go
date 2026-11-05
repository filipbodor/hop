package main

import (
	"os"

	"github.com/filipbodor/hop/internal/cli"
	"golang.org/x/term"
)

var version = "0.2.0"

func main() {
	app := cli.App{
		In: os.Stdin, Out: os.Stdout, Err: os.Stderr,
		Interactive: term.IsTerminal(int(os.Stdin.Fd())), Version: version,
	}
	os.Exit(app.Run(os.Args[1:]))
}
