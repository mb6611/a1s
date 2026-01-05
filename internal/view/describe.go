// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
	"gopkg.in/yaml.v3"
)

// Describe displays detailed information about a specific AWS resource.
type Describe struct {
	*tview.TextView

	resourceID *dao.ResourceID
	factory    dao.Factory
	path       string
	format     string
	rawData    interface{}
	actions    *ui.KeyActions
	backFn     func()
	wrapOn     bool
	app        *App
}

// NewDescribe creates a new resource detail view.
func NewDescribe(rid *dao.ResourceID) *Describe {
	d := &Describe{
		TextView:   tview.NewTextView(),
		resourceID: rid,
		format:     "yaml", // Default to YAML for AWS-style output
		actions:    ui.NewKeyActions(),
	}

	d.SetDynamicColors(true)
	d.SetWrap(false)
	d.SetWordWrap(false)
	d.SetScrollable(true)
	d.SetBorder(true)
	d.SetBorderPadding(0, 0, 1, 1)
	d.SetBorderColor(tcell.ColorAqua)

	return d
}

// Init initializes the describe view.
func (d *Describe) Init(ctx context.Context) error {
	d.bindKeys()
	d.SetInputCapture(d.keyboard)
	return nil
}

// Start starts the describe view.
func (d *Describe) Start() {
	d.Refresh()
}

// Stop stops the describe view.
func (d *Describe) Stop() {
	d.Clear()
}

// Name returns the view name.
func (d *Describe) Name() string {
	return "describe"
}

// Hints returns the menu hints for this view.
func (d *Describe) Hints() ui.MenuHints {
	return d.actions.Hints()
}

// SetFactory sets the AWS factory for fetching data.
func (d *Describe) SetFactory(f dao.Factory) {
	d.factory = f
}

// SetPath sets the resource path/ARN to describe.
func (d *Describe) SetPath(path string) {
	d.path = path
	d.updateTitle()
}

// SetBackFn sets the callback for back navigation.
func (d *Describe) SetBackFn(fn func()) {
	d.backFn = fn
}

// SetApp sets the application instance.
func (d *Describe) SetApp(app *App) {
	d.app = app
}

// Refresh reloads the resource content.
func (d *Describe) Refresh() {
	d.Clear()

	if d.path == "" {
		d.SetText("[red::]No resource selected[-::]")
		return
	}

	// Fetch resource data
	if err := d.fetchData(); err != nil {
		d.SetText(fmt.Sprintf("[red::]Error fetching resource: %v[-::]", err))
		return
	}

	d.SetText(d.generateContent())
	d.updateTitle()
	d.ScrollToBeginning()
}

// fetchData retrieves the resource data from AWS.
func (d *Describe) fetchData() error {
	if d.factory == nil {
		return fmt.Errorf("no factory available")
	}

	accessor, err := dao.AccessorFor(d.factory, d.resourceID)
	if err != nil {
		return fmt.Errorf("no accessor for %s: %w", d.resourceID.String(), err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	obj, err := accessor.Get(ctx, d.path)
	if err != nil {
		return err
	}

	d.rawData = obj.GetRaw()
	return nil
}

// bindKeys sets up key bindings for the view.
func (d *Describe) bindKeys() {
	d.actions.Clear()

	d.actions.Bulk(ui.KeyMap{
		ui.KeyY:        ui.NewKeyAction("YAML", d.formatCmd("yaml"), true),
		ui.KeyJ:        ui.NewKeyAction("JSON", d.formatCmd("json"), true),
		ui.KeyW:        ui.NewKeyAction("Wrap", d.toggleWrap, true),
		ui.KeyE:        ui.NewKeyAction("Edit", d.edit, true),
		tcell.KeyEsc:   ui.NewKeyAction("Back", d.backCmd, true),
		ui.KeyQ:        ui.NewSharedKeyAction("Back", d.backCmd, false),
	})
}

// toggleWrap toggles word wrap on/off.
func (d *Describe) toggleWrap(evt *tcell.EventKey) *tcell.EventKey {
	d.wrapOn = !d.wrapOn
	d.SetWrap(d.wrapOn)
	d.SetWordWrap(d.wrapOn)
	return nil
}

// keyboard handles keyboard input.
func (d *Describe) keyboard(evt *tcell.EventKey) *tcell.EventKey {
	if evt == nil {
		return nil
	}

	// Handle scrolling
	switch evt.Key() {
	case tcell.KeyDown:
		row, _ := d.GetScrollOffset()
		d.ScrollTo(row+1, 0)
		return nil
	case tcell.KeyUp:
		row, _ := d.GetScrollOffset()
		if row > 0 {
			d.ScrollTo(row-1, 0)
		}
		return nil
	case tcell.KeyPgDn:
		row, _ := d.GetScrollOffset()
		d.ScrollTo(row+20, 0)
		return nil
	case tcell.KeyPgUp:
		row, _ := d.GetScrollOffset()
		if row > 20 {
			d.ScrollTo(row-20, 0)
		} else {
			d.ScrollTo(0, 0)
		}
		return nil
	case tcell.KeyHome:
		d.ScrollToBeginning()
		return nil
	case tcell.KeyEnd:
		d.ScrollToEnd()
		return nil
	}

	// Handle vim-style scrolling
	if evt.Key() == tcell.KeyRune {
		row, _ := d.GetScrollOffset()
		switch evt.Rune() {
		case 'j':
			d.ScrollTo(row+1, 0)
			return nil
		case 'k':
			if row > 0 {
				d.ScrollTo(row-1, 0)
			}
			return nil
		case 'g':
			d.ScrollToBeginning()
			return nil
		case 'G':
			d.ScrollToEnd()
			return nil
		}
	}

	key := evt.Key()
	if key == tcell.KeyRune {
		key = tcell.Key(evt.Rune())
	}

	if action, ok := d.actions.Get(key); ok {
		return action.Action(evt)
	}

	return evt
}

// formatCmd returns a handler for format switching.
func (d *Describe) formatCmd(format string) ui.ActionHandler {
	return func(evt *tcell.EventKey) *tcell.EventKey {
		d.format = format
		d.Clear()
		d.SetText(d.generateContent())
		d.updateTitle()
		d.ScrollToBeginning()
		return nil
	}
}

// backCmd handles going back to the previous view.
func (d *Describe) backCmd(evt *tcell.EventKey) *tcell.EventKey {
	if d.backFn != nil {
		d.backFn()
	}
	return nil
}

// updateTitle updates the view title with current context.
func (d *Describe) updateTitle() {
	format := strings.ToUpper(d.format)
	title := fmt.Sprintf(" %s/%s [%s] ", d.resourceID.String(), d.path, format)
	d.SetTitle(title)
}

// generateContent generates the display content based on format.
func (d *Describe) generateContent() string {
	if d.rawData == nil {
		return "[red::]No data available[-::]"
	}

	switch d.format {
	case "json":
		return d.generateJSON()
	default:
		return d.generateYAML()
	}
}

// generateYAML generates YAML format output matching AWS CLI style with syntax highlighting.
func (d *Describe) generateYAML() string {
	// Convert to a clean map for YAML output
	data := d.toCleanMap(d.rawData)

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("[red::]# Error generating YAML: %v[-::]", err)
	}

	// Apply syntax highlighting
	return d.highlightYAML(string(out))
}

// highlightYAML applies syntax highlighting to YAML content.
func (d *Describe) highlightYAML(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if line == "" {
			result.WriteString("\n")
			continue
		}

		// Find the key part (before the colon)
		colonIdx := strings.Index(line, ":")
		if colonIdx > 0 {
			// Check if this is a key: value line or just a key:
			key := line[:colonIdx+1]
			value := ""
			if colonIdx+1 < len(line) {
				value = line[colonIdx+1:]
			}

			// Calculate indent (leading spaces or dashes)
			indent := ""
			keyStart := 0
			for i, ch := range key {
				if ch == ' ' || ch == '-' {
					indent += string(ch)
					keyStart = i + 1
				} else {
					break
				}
			}

			actualKey := key[keyStart:]

			// Color the key in teal/cyan, value based on type
			// derailed/tview format: [fg:bg:attrs]text[-::-]
			if value == "" || strings.TrimSpace(value) == "" {
				// Key only (nested object starts)
				result.WriteString(fmt.Sprintf("%s[aqua::]%s[-::]\n", indent, actualKey))
			} else {
				// Key: value pair
				coloredValue := d.colorizeValue(strings.TrimSpace(value))
				result.WriteString(fmt.Sprintf("%s[aqua::]%s[-::] %s\n", indent, actualKey, coloredValue))
			}
		} else if strings.HasPrefix(strings.TrimSpace(line), "-") {
			// List item without key
			indent := ""
			for _, ch := range line {
				if ch == ' ' {
					indent += " "
				} else {
					break
				}
			}
			rest := strings.TrimPrefix(line, indent)
			result.WriteString(fmt.Sprintf("%s%s\n", indent, rest))
		} else {
			// Plain value or continuation
			result.WriteString(line + "\n")
		}
	}

	return result.String()
}

// colorizeValue applies color based on value type.
// Uses derailed/tview format: [fg:bg:attrs]text[-::-]
func (d *Describe) colorizeValue(value string) string {
	trimmed := strings.Trim(value, "\"'")

	// Boolean values - green for true, red for false
	if trimmed == "true" || trimmed == "True" {
		return "[green::]" + value + "[-::]"
	}
	if trimmed == "false" || trimmed == "False" {
		return "[red::]" + value + "[-::]"
	}

	// Numbers - purple/fuchsia
	if _, err := fmt.Sscanf(trimmed, "%d", new(int)); err == nil {
		return "[fuchsia::]" + value + "[-::]"
	}
	if _, err := fmt.Sscanf(trimmed, "%f", new(float64)); err == nil {
		return "[fuchsia::]" + value + "[-::]"
	}

	// Null/nil - gray
	if trimmed == "null" || trimmed == "nil" || trimmed == "~" {
		return "[gray::]" + value + "[-::]"
	}

	// Status values
	lower := strings.ToLower(trimmed)
	if lower == "running" || lower == "active" || lower == "available" || lower == "attached" || lower == "enabled" {
		return "[green::]" + value + "[-::]"
	}
	if lower == "stopped" || lower == "terminated" || lower == "failed" || lower == "error" || lower == "disabled" {
		return "[red::]" + value + "[-::]"
	}
	if lower == "pending" || lower == "starting" || lower == "stopping" || lower == "updating" {
		return "[yellow::]" + value + "[-::]"
	}

	// Default - no color change
	return value
}

// generateJSON generates JSON format output.
func (d *Describe) generateJSON() string {
	data := d.toCleanMap(d.rawData)

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("// Error generating JSON: %v", err)
	}

	return string(out)
}

// toCleanMap converts AWS SDK structs to clean maps for serialization.
// This handles AWS SDK's pointer-heavy types and produces clean output.
func (d *Describe) toCleanMap(obj interface{}) interface{} {
	if obj == nil {
		return nil
	}

	val := reflect.ValueOf(obj)

	// Dereference pointers
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Struct:
		// Handle time.Time specially
		if t, ok := val.Interface().(time.Time); ok {
			return t.Format(time.RFC3339)
		}

		result := make(map[string]interface{})
		typ := val.Type()

		for i := 0; i < val.NumField(); i++ {
			field := typ.Field(i)
			fieldVal := val.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Skip nil pointers
			if fieldVal.Kind() == reflect.Ptr && fieldVal.IsNil() {
				continue
			}

			// Skip empty slices
			if fieldVal.Kind() == reflect.Slice && fieldVal.Len() == 0 {
				continue
			}

			// Skip empty maps
			if fieldVal.Kind() == reflect.Map && fieldVal.Len() == 0 {
				continue
			}

			// Skip zero-value strings for cleaner output
			if fieldVal.Kind() == reflect.String && fieldVal.String() == "" {
				continue
			}

			// Get field name (use JSON tag if available for AWS SDK compatibility)
			name := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" && parts[0] != "-" {
					name = parts[0]
				}
			}

			cleanVal := d.toCleanMap(fieldVal.Interface())
			if cleanVal != nil {
				result[name] = cleanVal
			}
		}

		if len(result) == 0 {
			return nil
		}
		return result

	case reflect.Slice:
		var result []interface{}
		for i := 0; i < val.Len(); i++ {
			item := d.toCleanMap(val.Index(i).Interface())
			if item != nil {
				result = append(result, item)
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result

	case reflect.Map:
		result := make(map[string]interface{})
		iter := val.MapRange()
		for iter.Next() {
			key := fmt.Sprintf("%v", iter.Key().Interface())
			cleanVal := d.toCleanMap(iter.Value().Interface())
			if cleanVal != nil {
				result[key] = cleanVal
			}
		}
		if len(result) == 0 {
			return nil
		}
		// Sort keys for consistent output
		return d.sortedMap(result)

	case reflect.String:
		s := val.String()
		if s == "" {
			return nil
		}
		return s

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint()

	case reflect.Float32, reflect.Float64:
		return val.Float()

	case reflect.Bool:
		return val.Bool()

	default:
		// For other types, try to get the interface value
		if val.CanInterface() {
			return val.Interface()
		}
		return nil
	}
}

// sortedMap returns the map with keys in sorted order for consistent output.
func (d *Describe) sortedMap(m map[string]interface{}) map[string]interface{} {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make(map[string]interface{})
	for _, k := range keys {
		result[k] = m[k]
	}
	return result
}

// edit opens the resource for editing via Cloud Control API.
func (d *Describe) edit(evt *tcell.EventKey) *tcell.EventKey {
	if d.resourceID == nil {
		return nil
	}

	// Check if resource type is supported by Cloud Control
	_, ok := dao.GetCloudFormationType(d.resourceID)
	if !ok {
		// Resource type not supported - silently ignore if no app
		if d.app != nil {
			d.app.Flash().Warnf("Edit not supported for %s", d.resourceID.String())
		}
		return nil
	}

	// Validate dependencies
	if d.factory == nil {
		if d.app != nil {
			d.app.Flash().Err(errors.New("no factory available"))
		}
		return nil
	}

	client := d.factory.Client()
	if client == nil {
		if d.app != nil {
			d.app.Flash().Err(errors.New("no AWS client available"))
		}
		return nil
	}

	if d.app == nil {
		// Cannot edit without app (needed for suspend)
		return nil
	}

	// Get region - extract from path for regional resources (format: region/id)
	region := ""
	if d.resourceID.Service == "ec2" || d.resourceID.Service == "vpc" || d.resourceID.Service == "eks" {
		if idx := strings.Index(d.path, "/"); idx > 0 {
			region = d.path[:idx]
		}
	}
	if region == "" {
		region = d.factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Perform edit
	ctx := context.Background()
	err := EditResource(ctx, d.app.Application, client, d.resourceID, d.path, region)

	if err != nil {
		if errors.Is(err, ErrEditorCancelled) {
			d.app.Flash().Info("Edit cancelled")
		} else if errors.Is(err, ErrNoChanges) {
			d.app.Flash().Info("No changes detected")
		} else {
			d.app.Flash().Errf("Edit failed: %v", err)
		}
		return nil
	}

	// Success - refresh the view
	d.app.Flash().Info("Resource updated successfully")
	d.Refresh()

	return nil
}
