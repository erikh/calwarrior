package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
)

type cliContext struct {
	*cli.Context
}

func (ctx *cliContext) makeTags() (taskWarriorTags, error) {
	slice := ctx.StringSlice("tag")
	if len(slice) == 0 {
		return nil, errors.New("at least one tag is required")
	}

	return taskWarriorTags(slice), nil
}

func (ctx *cliContext) getTimeWindow() (time.Time, time.Time) {
	t1 := time.Now().Add(-time.Hour * 24)
	t2 := t1.Add(ctx.Duration("duration"))

	return t1, t2
}

func (ctx *cliContext) run() error {
	tw, err := findTaskwarrior()
	if err != nil {
		return err
	}

	cal, err := getCalendarClient()
	if err != nil {
		return fmt.Errorf("Trouble contacting google calendar: %w", err)
	}

	m := newMerge(ctx, tw, cal)
	if err := m.run(); err != nil {
		m.log.Error(err)
	}

	return nil
}
