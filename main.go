package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()
	app.Authors = []*cli.Author{{Name: "Erik Hollensbe", Email: "erik+github@hollensbe.org"}}
	app.Usage = "Synchronize Google Calendar and Taskwarrior"
	app.UsageText = filepath.Base(os.Args[0]) + " [--flags or help]"

	app.Flags = []cli.Flag{
		&cli.DurationFlag{
			Name:    "duration",
			Aliases: []string{"d"},
			Usage:   "Upcoming items to monitor in google calendar. Keeping this small and polling frequently is better",
			Value:   7 * 24 * time.Hour,
		},
		&cli.StringSliceFlag{
			Name:    "tag",
			Aliases: []string{"t"},
			Usage:   "Tag new items coming from google calendar with the specified tag(s).",
			Value:   cli.NewStringSlice("calendar"),
		},
		&cli.BoolFlag{
			Name:    "color",
			Aliases: []string{"c"},
			Usage:   "Turns on color support (on by default)",
			EnvVars: []string{"CALWARRIOR_COLOR"},
			Value:   true,
		},
	}

	app.Action = run

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx *cli.Context) error {
	if !ctx.Bool("color") {
		color.NoColor = true
	}

	cli := &cliContext{ctx}
	return cli.run()
}
