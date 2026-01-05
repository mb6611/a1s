// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"fmt"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// ProfileSwitcher displays and allows switching between AWS profiles.
type ProfileSwitcher struct {
	*tview.Table

	app      *App
	factory  dao.Factory
	profiles []string
	current  string
}

// NewProfileSwitcher creates a new profile switcher view.
func NewProfileSwitcher(app *App) *ProfileSwitcher {
	p := &ProfileSwitcher{
		Table: tview.NewTable(),
		app:   app,
	}

	p.SetBorder(true)
	p.SetTitle(" Profiles ")
	p.SetTitleAlign(tview.AlignCenter)
	p.SetBorderColor(tcell.ColorAqua)
	p.SetBackgroundColor(tcell.ColorDefault)
	p.SetSelectable(true, false)
	p.SetFixed(1, 0)

	return p
}

// Init initializes the profile switcher.
func (p *ProfileSwitcher) Init(ctx context.Context) error {
	p.SetInputCapture(p.keyboard)
	p.loadProfiles()
	return nil
}

// Start begins the view lifecycle.
func (p *ProfileSwitcher) Start() {
	p.loadProfiles()
}

// Stop ends the view lifecycle.
func (p *ProfileSwitcher) Stop() {}

// SetFactory sets the AWS factory.
func (p *ProfileSwitcher) SetFactory(f dao.Factory) {
	p.factory = f
}

// Name returns the view name.
func (p *ProfileSwitcher) Name() string {
	return "profile"
}

// Hints returns menu hints.
func (p *ProfileSwitcher) Hints() ui.MenuHints {
	return ui.MenuHints{
		{Mnemonic: "enter", Description: "Switch to profile", Visible: true},
		{Mnemonic: "esc", Description: "Back", Visible: true},
	}
}

// keyboard handles keyboard input.
func (p *ProfileSwitcher) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	key := evt.Key()
	row, col := p.GetSelection()
	rowCount := p.GetRowCount()

	// Vim navigation
	if key == tcell.KeyRune {
		switch evt.Rune() {
		case 'j':
			if row < rowCount-1 {
				p.Select(row+1, col)
			}
			return nil
		case 'k':
			if row > 1 {
				p.Select(row-1, col)
			}
			return nil
		case 'g':
			if rowCount > 1 {
				p.Select(1, col)
			}
			return nil
		case 'G':
			if rowCount > 1 {
				p.Select(rowCount-1, col)
			}
			return nil
		}
	}

	switch key {
	case tcell.KeyEnter:
		p.selectProfile()
		return nil
	case tcell.KeyDown:
		if row < rowCount-1 {
			p.Select(row+1, col)
		}
		return nil
	case tcell.KeyUp:
		if row > 1 {
			p.Select(row-1, col)
		}
		return nil
	}

	return evt
}

// loadProfiles loads available profiles.
func (p *ProfileSwitcher) loadProfiles() {
	p.Clear()

	// Build header
	headers := []string{"", "PROFILE", "REGION", "STATUS"}
	for col, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetExpansion(1)
		p.SetCell(0, col, cell)
	}

	if p.factory == nil {
		p.showNoData("No factory available")
		return
	}

	// Get current profile
	p.current = p.factory.Profile()

	// Get profiles from factory's client
	client := p.factory.Client()
	if client == nil {
		p.showNoData("No AWS client")
		return
	}

	p.profiles = client.ProfileNames()
	if len(p.profiles) == 0 {
		p.showNoData("No profiles found")
		return
	}

	// Build rows
	for i, name := range p.profiles {
		row := i + 1

		// Active indicator
		indicator := ""
		indicatorColor := tcell.ColorDefault
		if name == p.current {
			indicator = "●"
			indicatorColor = tcell.ColorGreen
		}
		indicatorCell := tview.NewTableCell(indicator).
			SetTextColor(indicatorColor).
			SetAlign(tview.AlignCenter).
			SetExpansion(0)
		p.SetCell(row, 0, indicatorCell)

		// Profile name
		nameColor := tcell.ColorWhite
		if name == p.current {
			nameColor = tcell.ColorGreen
		}
		nameCell := tview.NewTableCell(name).
			SetTextColor(nameColor).
			SetExpansion(1).
			SetReference(name)
		p.SetCell(row, 1, nameCell)

		// Region
		region := client.ProfileRegion(name)
		if region == "" {
			region = "(default)"
		}
		regionCell := tview.NewTableCell(region).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1)
		p.SetCell(row, 2, regionCell)

		// Status
		status := ""
		if name == p.current {
			status = "active"
		}
		statusCell := tview.NewTableCell(status).
			SetTextColor(tcell.ColorGreen).
			SetExpansion(1)
		p.SetCell(row, 3, statusCell)
	}

	// Update title with count
	p.SetTitle(fmt.Sprintf(" Profiles [%d] ", len(p.profiles)))

	// Select first data row
	if p.GetRowCount() > 1 {
		p.Select(1, 0)
	}
}

// showNoData displays a message when no profiles found.
func (p *ProfileSwitcher) showNoData(msg string) {
	cell := tview.NewTableCell(msg).
		SetTextColor(tcell.ColorGray).
		SetAlign(tview.AlignCenter).
		SetSelectable(false)
	p.SetCell(1, 0, cell)
}

// selectProfile switches to the selected profile.
func (p *ProfileSwitcher) selectProfile() {
	row, _ := p.GetSelection()
	if row == 0 || row > len(p.profiles) {
		return
	}

	profileName := p.profiles[row-1]
	if profileName == p.current {
		p.app.Flash().Infof("Already using profile: %s", profileName)
		return
	}

	// Switch profile
	if err := p.app.SwitchProfile(profileName); err != nil {
		p.app.Flash().Errf("Failed to switch profile: %v", err)
		return
	}

	p.app.Flash().Infof("Switched to profile: %s", profileName)
	p.current = profileName
	p.loadProfiles() // Refresh to update active indicator
}

// SetFilter implements the filterable interface (no-op for profiles).
func (p *ProfileSwitcher) SetFilter(filter string) {
	// Could implement filtering if needed
}

// UpdateUI updates the view with new data.
func (p *ProfileSwitcher) UpdateUI(data *model1.TableData) {
	// Not used - we load directly from profile manager
}
