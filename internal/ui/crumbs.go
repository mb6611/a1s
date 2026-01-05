// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"strings"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// Crumbs represents user breadcrumbs.
type Crumbs struct {
	*tview.TextView

	stack *Stack
}

// NewCrumbs returns a new breadcrumb view.
func NewCrumbs() *Crumbs {
	c := &Crumbs{
		stack:    NewStack(),
		TextView: tview.NewTextView(),
	}
	c.SetBackgroundColor(tcell.ColorDefault)
	c.SetTextAlign(tview.AlignLeft)
	c.SetBorderPadding(0, 0, 1, 1)
	c.SetDynamicColors(true)

	return c
}

// StackPushed indicates a new item was added.
func (c *Crumbs) StackPushed(comp Component) {
	c.stack.Push(comp)
	c.refresh(c.stack.Flatten())
}

// StackPopped indicates an item was deleted.
func (c *Crumbs) StackPopped(_, _ Component) {
	c.stack.Pop()
	c.refresh(c.stack.Flatten())
}

// StackTop indicates the top of the stack.
func (*Crumbs) StackTop(Component) {}

// Refresh updates view with new crumbs.
func (c *Crumbs) refresh(crumbs []string) {
	c.Clear()
	last := len(crumbs) - 1

	for i, crumb := range crumbs {
		if i == last {
			// Active crumb - bright yellow
			_, _ = fmt.Fprintf(c, "[yellow:black:b] <%s> [-:-:-] ",
				strings.ReplaceAll(strings.ToLower(crumb), " ", ""))
		} else {
			// Inactive crumb - dim
			_, _ = fmt.Fprintf(c, "[gray::-] <%s> [-:-:-] ",
				strings.ReplaceAll(strings.ToLower(crumb), " ", ""))
		}
	}
}
