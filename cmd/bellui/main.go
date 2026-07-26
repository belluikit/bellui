// Run a command line helper for various Bellui tools.
package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/urfave/cli/v2"

	"github.com/belluikit/bellui/cmd/bellui/internal/commands"
)

func main() {
	app := &cli.App{
		Name:        "bellui",
		Usage:       "A command line helper for various Bellui tools.",
		Description: "The bellui command provides tooling for bellui applications and to assist in their development.",
		Commands: []*cli.Command{
			commands.Init(),
			commands.Env(),
			commands.Build(),
			commands.Package(),
			commands.Release(),
			commands.Install(),
			commands.Serve(),
			commands.Translate(),
			commands.Version(),
			commands.Bundle(),
		},
	}

	info, ok := debug.ReadBuildInfo()
	if ok {
		app.Version = info.Main.Version
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
