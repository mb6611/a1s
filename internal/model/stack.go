package model

import (
	"sync"

	"github.com/a1s/a1s/internal/model1"
)

// Component represents a UI component
type Component interface {
	Name() string
	Stop()
}

// TableListener represents a table model listener.
type TableListener interface {
	// TableNoData notifies listener no data was found.
	TableNoData(*model1.TableData)

	// TableDataChanged notifies the model data changed.
	TableDataChanged(*model1.TableData)

	// TableLoadFailed notifies the load failed.
	TableLoadFailed(error)
}

// StackListener listens to stack events
type StackListener interface {
	StackPushed(Component)
	StackPopped(old, new Component)
	StackTop(Component)
}

// Stack manages a component stack
type Stack struct {
	components []Component
	listeners  []StackListener
	mx         sync.RWMutex
}

// NewStack returns a new stack
func NewStack() *Stack {
	return &Stack{}
}

// AddListener adds a stack listener
func (s *Stack) AddListener(l StackListener) {
	s.listeners = append(s.listeners, l)
	if !s.Empty() {
		l.StackTop(s.Top())
	}
}

// RemoveListener removes a stack listener
func (s *Stack) RemoveListener(l StackListener) {
	victim := -1
	for i, lis := range s.listeners {
		if lis == l {
			victim = i
			break
		}
	}
	if victim == -1 {
		return
	}
	s.listeners = append(s.listeners[:victim], s.listeners[victim+1:]...)
}

// Push adds a component to the stack
func (s *Stack) Push(c Component) {
	if top := s.Top(); top != nil {
		top.Stop()
	}

	s.mx.Lock()
	s.components = append(s.components, c)
	s.mx.Unlock()

	for _, l := range s.listeners {
		l.StackPushed(c)
		l.StackTop(c)
	}
}

// Pop removes the top component
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

	new := s.Top()
	for _, l := range s.listeners {
		l.StackPopped(c, new)
		if new != nil {
			l.StackTop(new)
		}
	}

	return c, true
}

// Top returns the top component
func (s *Stack) Top() Component {
	if s.Empty() {
		return nil
	}

	s.mx.RLock()
	defer s.mx.RUnlock()
	return s.components[len(s.components)-1]
}

// Empty checks if stack is empty
func (s *Stack) Empty() bool {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return len(s.components) == 0
}

// IsLast indicates if stack only has one item left
func (s *Stack) IsLast() bool {
	return len(s.components) == 1
}

// Clear removes all components using pops
func (s *Stack) Clear() {
	for range s.components {
		s.Pop()
	}
}

// Peek returns stack state
func (s *Stack) Peek() []Component {
	s.mx.RLock()
	defer s.mx.RUnlock()

	return s.components
}

// Previous returns the component below the top
func (s *Stack) Previous() Component {
	if s.Empty() {
		return nil
	}
	if s.IsLast() {
		return s.Top()
	}

	return s.components[len(s.components)-2]
}

// Flatten returns all component names as a slice
func (s *Stack) Flatten() []string {
	s.mx.RLock()
	defer s.mx.RUnlock()

	ss := make([]string, len(s.components))
	for i, c := range s.components {
		ss[i] = c.Name()
	}
	return ss
}
