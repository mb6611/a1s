package ui

import (
	"github.com/derailed/tview"
)

// Pages represents a page manager
type Pages struct {
	*tview.Pages
	stack    []string
	pageMap  map[string]tview.Primitive
}

// NewPages returns a new pages manager
func NewPages() *Pages {
	return &Pages{
		Pages:   tview.NewPages(),
		stack:   make([]string, 0),
		pageMap: make(map[string]tview.Primitive),
	}
}

// Push adds a new page
func (p *Pages) Push(name string, page tview.Primitive) {
	p.stack = append(p.stack, name)
	p.pageMap[name] = page
	p.AddPage(name, page, true, true)
	p.SwitchToPage(name)
}

// Pop removes the current page
func (p *Pages) Pop() (string, bool) {
	if len(p.stack) == 0 {
		return "", false
	}

	name := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	delete(p.pageMap, name)
	p.RemovePage(name)

	if len(p.stack) > 0 {
		top := p.stack[len(p.stack)-1]
		p.SwitchToPage(top)
		return top, true
	}

	return "", true
}

// Current returns the current page name
func (p *Pages) Current() string {
	if len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

// CurrentPage returns the current page primitive
func (p *Pages) CurrentPage() tview.Primitive {
	name := p.Current()
	if name == "" {
		return nil
	}
	return p.pageMap[name]
}

// StackSize returns the stack depth
func (p *Pages) StackSize() int {
	return len(p.stack)
}

// ClearStack clears all pages
func (p *Pages) ClearStack() {
	for _, name := range p.stack {
		p.RemovePage(name)
	}
	p.stack = p.stack[:0]
	p.pageMap = make(map[string]tview.Primitive)
}
