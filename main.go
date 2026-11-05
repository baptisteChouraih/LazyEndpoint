package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/baptisteChouraih/LazyEndpoint/internal/spec"
	"github.com/baptisteChouraih/LazyEndpoint/internal/tui"
)

var version = "0.1.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "lazyendpoint: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("lazyendpoint", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = usage

	tag := fs.String("tag", "", "only show endpoints carrying this tag")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println("lazyendpoint " + version)
		return nil
	}
	if fs.NArg() != 1 {
		usage()
		return errors.New("expected exactly one spec file or URL")
	}

	location := fs.Arg(0)
	doc, err := spec.Load(location)
	if err != nil {
		return err
	}

	model := tui.New(doc, tui.Options{InitialTag: *tag})
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

func usage() {
	fmt.Fprint(os.Stderr, `lazyendpoint -- browse an OpenAPI spec in your terminal

Usage:
  lazyendpoint <spec.yaml|spec.json|https://...> [flags]

Flags:
  -tag string   only show endpoints carrying this tag
  -version      print the version and exit

Examples:
  lazyendpoint openapi.yaml
  lazyendpoint https://petstore3.swagger.io/api/v3/openapi.json
  lazyendpoint api.json -tag pets
`)
}
