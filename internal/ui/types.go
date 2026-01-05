package ui

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a1s/a1s/internal/model"
	"github.com/a1s/a1s/internal/model1"
	"github.com/derailed/tview"
)

// Namespaceable tracks namespaces.
type Namespaceable interface {
	// ClusterWide returns true if the model represents resource in all namespaces.
	ClusterWide() bool

	// GetNamespace returns the model namespace.
	GetNamespace() string

	// SetNamespace changes the model namespace.
	SetNamespace(string)

	// InNamespace check if current namespace matches models.
	InNamespace(string) bool
}

// Lister tracks resource getter.
type Lister interface {
	// Get returns a resource instance.
	Get(ctx context.Context, path string) (interface{}, error)
}

// Tabular represents a tabular model.
type Tabular interface {
	Namespaceable
	Lister

	// SetInstance sets parent resource path.
	SetInstance(string)

	// Empty returns true if model has no data.
	Empty() bool

	// RowCount returns the model data count.
	RowCount() int

	// Peek returns current model data.
	Peek() *model1.TableData

	// Watch watches a given resource for changes.
	Watch(context.Context) error

	// Refresh forces a new refresh.
	Refresh(context.Context) error

	// SetRefreshRate sets the model watch loop rate.
	SetRefreshRate(time.Duration)

	// AddListener registers a model listener.
	AddListener(model.TableListener)

	// RemoveListener unregister a model listener.
	RemoveListener(model.TableListener)
}

// MenuHint represents a keyboard mnemonic.
type MenuHint struct {
	Mnemonic    string
	Description string
	Visible     bool
}

// IsBlank checks if menu hint is a placeholder.
func (m MenuHint) IsBlank() bool {
	return m.Mnemonic == "" && m.Description == "" && !m.Visible
}

// MenuHints represents a collection of hints.
type MenuHints []MenuHint

// Len returns the hints length.
func (h MenuHints) Len() int {
	return len(h)
}

// Swap swaps two elements.
func (h MenuHints) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Less returns true if first hint is less than second.
func (h MenuHints) Less(i, j int) bool {
	n, err1 := strconv.Atoi(h[i].Mnemonic)
	m, err2 := strconv.Atoi(h[j].Mnemonic)
	if err1 == nil && err2 == nil {
		return n < m
	}
	if err1 == nil && err2 != nil {
		return true
	}
	if err1 != nil && err2 == nil {
		return false
	}
	return h[i].Description < h[j].Description
}

// Hinter represent a menu mnemonic provider.
type Hinter interface {
	// Hints returns a collection of menu hints.
	Hints() MenuHints
}

// Primitive represents a UI primitive.
type Primitive interface {
	tview.Primitive

	// Name returns the view name.
	Name() string
}

// Igniter represents a runnable view.
type Igniter interface {
	// Init initializes a component.
	Init(ctx context.Context) error

	// Start starts a component.
	Start()

	// Stop terminates a component.
	Stop()
}

// Component represents a ui component.
type Component interface {
	Primitive
	Igniter
	Hinter
}

// StackListener represents a stack listener.
type StackListener interface {
	// StackPushed indicates a new item was added.
	StackPushed(Component)

	// StackPopped indicates an item was deleted
	StackPopped(old, new Component)

	// StackTop indicates the top of the stack
	StackTop(Component)
}

// StackAction represents an action on the stack.
type StackAction int

const (
	// StackPush denotes an add on the stack.
	StackPush StackAction = 1 << iota

	// StackPop denotes a delete on the stack.
	StackPop
)

// Stack represents a stack of components.
type Stack struct {
	components []Component
	listeners  []StackListener
	mx         sync.RWMutex
}

// NewStack returns a new initialized stack.
func NewStack() *Stack {
	return &Stack{}
}

// Flatten returns a string representation of the component stack.
func (s *Stack) Flatten() []string {
	s.mx.RLock()
	defer s.mx.RUnlock()

	ss := make([]string, len(s.components))
	for i, c := range s.components {
		ss[i] = c.Name()
	}
	return ss
}

// AddListener registers a stack listener.
func (s *Stack) AddListener(l StackListener) {
	s.listeners = append(s.listeners, l)
	if !s.Empty() {
		l.StackTop(s.Top())
	}
}

// Push adds a new item.
func (s *Stack) Push(c Component) {
	if top := s.Top(); top != nil {
		top.Stop()
	}

	s.mx.Lock()
	s.components = append(s.components, c)
	s.mx.Unlock()
	s.notify(StackPush, c)
}

// Pop removed the top item and returns it.
func (s *Stack) Pop() (Component, bool) {
	if s.Empty() {
		return nil, false
	}

	var c Component
	s.mx.Lock()
	c = s.components[len(s.components)-1]
	c.Stop()
	s.components = s.components[:len(s.components)-1]
	s.mx.Unlock()

	s.notify(StackPop, c)

	return c, true
}

// Empty returns true if the stack is empty.
func (s *Stack) Empty() bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return len(s.components) == 0
}

// Top returns the top most item or nil if the stack is empty.
func (s *Stack) Top() Component {
	if s.Empty() {
		return nil
	}

	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.components[len(s.components)-1]
}

func (s *Stack) notify(a StackAction, c Component) {
	for _, l := range s.listeners {
		switch a {
		case StackPush:
			l.StackPushed(c)
		case StackPop:
			l.StackPopped(c, s.Top())
		}
	}
}

// TrimCell removes superfluous padding from a table cell.
func TrimCell(tv *SelectTable, row, col int) string {
	c := tv.GetCell(row, col)
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Text)
}
