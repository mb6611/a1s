package data

// DefaultView is the default resource view when starting the app
const DefaultView = "ec2"

// View represents the active view state
type View struct {
	Active string `yaml:"active"`
}

// NewView creates a View with default settings
func NewView() *View {
	return &View{
		Active: DefaultView,
	}
}

// Validate ensures the View has valid settings
func (v *View) Validate() {
	if v.Active == "" {
		v.Active = DefaultView
	}
}
