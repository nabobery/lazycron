package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/nabobery/lazycron/internal/app"
	"github.com/nabobery/lazycron/internal/cli"
	"github.com/nabobery/lazycron/internal/tui"
)

func main() {
	deps := cli.DefaultDeps()

	code := cli.Run(os.Args, os.Stdout, os.Stderr, deps)
	if code >= 0 {
		os.Exit(code)
	}

	// No subcommand matched — launch the TUI
	if err := runTUI(deps); err != nil {
		fmt.Fprintf(os.Stderr, "lazycron: %v\n", err)
		os.Exit(1)
	}
}

func runTUI(deps cli.Deps) error {
	applySvc := app.NewApplyService(deps.Client, deps.Source)
	scheduleSvc := deps.ScheduleSvc
	jobRunner := deps.Runner

	model := tui.NewModel(applySvc, scheduleSvc, jobRunner)

	if deps.Discoverer != nil {
		invSvc := app.NewInventoryService(applySvc, deps.Discoverer)
		model.SetInventoryService(invSvc)
	}

	if deps.LogsProvider != nil {
		model.SetLogsProvider(deps.LogsProvider)
	}

	p := tea.NewProgram(model)

	_, err := p.Run()
	return err
}
