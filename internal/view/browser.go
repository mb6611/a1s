// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
)

// ContextKey represents context key.
type ContextKey string

// Context keys for AWS resource browsing.
const (
	KeyFactory    ContextKey = "factory"
	KeyResourceID ContextKey = "resourceID"
	KeyRegion     ContextKey = "region"
	KeyProfile    ContextKey = "profile"
)

// Browser represents a generic AWS resource browser.
type Browser struct {
	*Table

	app      *App
	factory  dao.Factory
	accessor dao.Accessor
	region   string
	cancelFn context.CancelFunc
	pushFn   func(name string, c ui.Component)
	popFn    func()
	mx       sync.RWMutex
}

// NewBrowser returns a new AWS resource browser.
func NewBrowser(rid *dao.ResourceID) *Browser {
	return &Browser{
		Table: NewTable(rid),
	}
}

// SetApp sets the App reference for flash messages and editor suspension.
func (b *Browser) SetApp(a *App) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.app = a
}

// SetFactory sets the AWS factory for this browser.
func (b *Browser) SetFactory(f dao.Factory) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.factory = f
}

// SetPushFn sets the navigation push function.
func (b *Browser) SetPushFn(fn func(name string, c ui.Component)) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.pushFn = fn
}

// SetPopFn sets the navigation pop function.
func (b *Browser) SetPopFn(fn func()) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.popFn = fn
}

// GetFactory returns the AWS factory.
func (b *Browser) GetFactory() dao.Factory {
	b.mx.RLock()
	defer b.mx.RUnlock()
	return b.factory
}

// Init initializes the browser component.
func (b *Browser) Init(ctx context.Context) error {
	if err := b.Table.Init(ctx); err != nil {
		return err
	}

	b.bindKeys(b.Actions())
	return nil
}

// Start initializes browser updates.
func (b *Browser) Start() {
	b.Stop()

	model := b.GetModel()
	if model != nil {
		model.AddListener(b)
		if err := model.Watch(b.prepareContext()); err != nil {
			// Log error - App.Flash() will be added when App is available
		}
	} else if b.factory != nil {
		// Load real AWS data using the factory
		b.loadRealData()
	} else {
		// Show demo data if no factory is connected
		b.loadDemoData()
	}
	b.Table.Start()
}

// loadRealData fetches real AWS resources using the DAO.
func (b *Browser) loadRealData() {
	rid := b.GetResourceID()
	if rid == nil {
		return
	}

	// Get or create accessor
	b.mx.Lock()
	if b.accessor == nil {
		acc, err := dao.AccessorFor(b.factory, rid)
		if err != nil {
			b.mx.Unlock()
			// Fall back to demo data on error
			b.loadDemoData()
			return
		}
		b.accessor = acc
	}
	accessor := b.accessor
	factory := b.factory
	b.mx.Unlock()

	// Determine region to query
	region := b.GetRegion()
	if region == "" && factory != nil {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Fetch data from AWS
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objects, err := accessor.List(ctx, region)
	if err != nil {
		// Show table with error message based on error type
		data := model1.NewTableData()
		data.SetNamespace(region)
		data.SetHeader(b.headerForResource(rid))

		// Determine appropriate error message
		errMsg := b.friendlyError(err, rid)
		data.SetError(errMsg)

		b.UpdateUI(data)
		return
	}

	// Convert to TableData using renderer
	data := b.renderObjects(objects, region, rid)
	b.UpdateUI(data)
}

// renderObjects converts AWS objects to TableData.
func (b *Browser) renderObjects(objects []dao.AWSObject, region string, rid *dao.ResourceID) *model1.TableData {
	data := model1.NewTableData()
	data.SetNamespace(region)

	if len(objects) == 0 {
		return data
	}

	// Build header based on resource type
	header := b.headerForResource(rid)
	data.SetHeader(header)

	// Build rows
	for _, obj := range objects {
		row := b.rowForObject(obj, rid, header)
		data.RowEvents().Add(model1.NewRowEvent(model1.EventAdd, row))
	}

	return data
}

// headerForResource returns the header for a resource type.
func (b *Browser) headerForResource(rid *dao.ResourceID) model1.Header {
	switch rid.String() {
	case "ec2/instance":
		return model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "TYPE"},
			{Name: "STATE"},
			{Name: "AZ"},
			{Name: "PUBLIC IP"},
			{Name: "PRIVATE IP"},
		}
	case "s3/bucket":
		return model1.Header{
			{Name: "NAME"},
			{Name: "REGION"},
			{Name: "CREATED"},
		}
	case "vpc/securitygroup":
		return model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "VPC"},
			{Name: "DESCRIPTION"},
		}
	case "iam/user":
		return model1.Header{
			{Name: "NAME"},
			{Name: "USER ID"},
			{Name: "CREATED"},
			{Name: "LAST USED"},
		}
	case "iam/role":
		return model1.Header{
			{Name: "NAME"},
			{Name: "ROLE ID"},
			{Name: "CREATED"},
			{Name: "DESCRIPTION"},
		}
	default:
		return model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "REGION"},
		}
	}
}

// rowForObject converts an AWS object to a table row.
func (b *Browser) rowForObject(obj dao.AWSObject, rid *dao.ResourceID, header model1.Header) model1.Row {
	row := model1.NewRow(len(header))
	row.ID = obj.GetID()

	raw := obj.GetRaw()

	switch rid.String() {
	case "ec2/instance":
		row.Fields[0] = obj.GetID()
		row.Fields[1] = obj.GetName()
		// Extract instance type, state, AZ, IPs from raw
		if inst, ok := raw.(interface{ GetInstanceType() string }); ok {
			row.Fields[2] = inst.GetInstanceType()
		} else {
			row.Fields[2] = extractField(raw, "InstanceType")
		}
		row.Fields[3] = extractField(raw, "State.Name")
		row.Fields[4] = extractField(raw, "Placement.AvailabilityZone")
		row.Fields[5] = extractField(raw, "PublicIpAddress")
		row.Fields[6] = extractField(raw, "PrivateIpAddress")

	case "s3/bucket":
		row.Fields[0] = obj.GetName()
		row.Fields[1] = obj.GetRegion()
		if t := obj.GetCreatedAt(); t != nil {
			row.Fields[2] = t.Format("2006-01-02")
		} else {
			row.Fields[2] = "-"
		}

	case "vpc/securitygroup":
		row.Fields[0] = obj.GetID()
		row.Fields[1] = obj.GetName()
		row.Fields[2] = extractField(raw, "VpcId")
		row.Fields[3] = extractField(raw, "Description")

	case "iam/user":
		// Header: NAME, USER ID, CREATED, LAST USED
		row.ID = obj.GetName() // Use name as row ID for IAM
		row.Fields[0] = obj.GetName()
		row.Fields[1] = obj.GetID()
		if t := obj.GetCreatedAt(); t != nil {
			row.Fields[2] = t.Format("2006-01-02")
		} else {
			row.Fields[2] = "-"
		}
		row.Fields[3] = extractField(raw, "PasswordLastUsed")

	case "iam/role":
		// Header: NAME, ROLE ID, CREATED, DESCRIPTION
		row.ID = obj.GetName() // Use name as row ID for IAM
		row.Fields[0] = obj.GetName()
		row.Fields[1] = obj.GetID()
		if t := obj.GetCreatedAt(); t != nil {
			row.Fields[2] = t.Format("2006-01-02")
		} else {
			row.Fields[2] = "-"
		}
		row.Fields[3] = extractField(raw, "Description")

	default:
		row.Fields[0] = obj.GetID()
		row.Fields[1] = obj.GetName()
		row.Fields[2] = obj.GetRegion()
	}

	return row
}

// extractField extracts a field from a struct using reflection.
func extractField(obj interface{}, path string) string {
	if obj == nil {
		return "-"
	}

	// Handle EC2 types specifically
	switch v := obj.(type) {
	case interface{ String() string }:
		// For enum types like InstanceStateName
		return v.String()
	}

	// Use reflection for nested field access
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "-"
		}
		val = val.Elem()
	}

	parts := strings.Split(path, ".")
	for _, part := range parts {
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return "-"
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return "-"
		}
		val = val.FieldByName(part)
		if !val.IsValid() {
			return "-"
		}
	}

	// Handle pointer to string
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "-"
		}
		val = val.Elem()
	}

	// Handle stringer interface (like enum types)
	if val.CanInterface() {
		if s, ok := val.Interface().(fmt.Stringer); ok {
			return s.String()
		}
	}

	return fmt.Sprintf("%v", val.Interface())
}

// loadDemoData populates the table with sample data for testing.
func (b *Browser) loadDemoData() {
	rid := b.GetResourceID()
	if rid == nil {
		return
	}

	data := model1.NewTableData()
	data.SetNamespace("us-east-1")

	// Build header and rows based on resource type
	switch rid.String() {
	case "ec2/instance":
		data.SetHeader(model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "TYPE"},
			{Name: "STATE"},
			{Name: "AZ"},
			{Name: "PUBLIC IP"},
			{Name: "PRIVATE IP"},
		})
		rows := model1.NewRowEvents(3)
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "i-0123456789abcdef0",
			Fields: model1.Fields{"i-0123456789abcdef0", "web-server-1", "t3.micro", "running", "us-east-1a", "54.123.45.67", "10.0.1.10"},
		}))
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "i-0123456789abcdef1",
			Fields: model1.Fields{"i-0123456789abcdef1", "api-server", "t3.small", "running", "us-east-1b", "54.123.45.68", "10.0.2.20"},
		}))
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "i-0123456789abcdef2",
			Fields: model1.Fields{"i-0123456789abcdef2", "db-primary", "r5.large", "stopped", "us-east-1a", "-", "10.0.1.50"},
		}))
		for i := 0; i < rows.Len(); i++ {
			if re, ok := rows.At(i); ok {
				data.RowEvents().Add(re)
			}
		}

	case "s3/bucket":
		data.SetHeader(model1.Header{
			{Name: "NAME"},
			{Name: "REGION"},
			{Name: "CREATED"},
			{Name: "SIZE"},
		})
		rows := model1.NewRowEvents(2)
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "my-app-bucket",
			Fields: model1.Fields{"my-app-bucket", "us-east-1", "2024-01-15", "1.2 GB"},
		}))
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "backup-storage",
			Fields: model1.Fields{"backup-storage", "us-west-2", "2023-06-20", "45 GB"},
		}))
		for i := 0; i < rows.Len(); i++ {
			if re, ok := rows.At(i); ok {
				data.RowEvents().Add(re)
			}
		}

	case "vpc/securitygroup":
		data.SetHeader(model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "VPC"},
			{Name: "INBOUND"},
			{Name: "OUTBOUND"},
		})
		rows := model1.NewRowEvents(2)
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "sg-0123456789abcdef0",
			Fields: model1.Fields{"sg-0123456789abcdef0", "web-sg", "vpc-abc123", "3", "1"},
		}))
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "sg-0123456789abcdef1",
			Fields: model1.Fields{"sg-0123456789abcdef1", "default", "vpc-abc123", "1", "1"},
		}))
		for i := 0; i < rows.Len(); i++ {
			if re, ok := rows.At(i); ok {
				data.RowEvents().Add(re)
			}
		}

	default:
		data.SetHeader(model1.Header{
			{Name: "ID"},
			{Name: "NAME"},
			{Name: "STATUS"},
		})
		rows := model1.NewRowEvents(1)
		rows.Add(model1.NewRowEvent(model1.EventAdd, model1.Row{
			ID:     "demo-resource",
			Fields: model1.Fields{"demo-resource", "sample", "active"},
		}))
		for i := 0; i < rows.Len(); i++ {
			if re, ok := rows.At(i); ok {
				data.RowEvents().Add(re)
			}
		}
	}

	b.UpdateUI(data)
}

// Stop terminates browser updates.
func (b *Browser) Stop() {
	b.mx.Lock()
	if b.cancelFn != nil {
		b.cancelFn()
		b.cancelFn = nil
	}
	b.mx.Unlock()

	model := b.GetModel()
	if model != nil {
		model.RemoveListener(b)
	}
	b.Table.Stop()
}

// SetAccessor sets the data accessor for this browser.
func (b *Browser) SetAccessor(a dao.Accessor) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.accessor = a
}

// GetAccessor returns the current accessor.
func (b *Browser) GetAccessor() dao.Accessor {
	b.mx.RLock()
	defer b.mx.RUnlock()
	return b.accessor
}

// SetRegion sets the AWS region filter.
func (b *Browser) SetRegion(region string) {
	b.mx.Lock()
	defer b.mx.Unlock()
	b.region = cleanseRegion(region)
}

// GetRegion returns the current region filter.
func (b *Browser) GetRegion() string {
	b.mx.RLock()
	defer b.mx.RUnlock()
	return b.region
}

// Name returns the component name for breadcrumbs.
func (b *Browser) Name() string {
	rid := b.GetResourceID()
	if rid == nil {
		return "unknown"
	}
	return rid.Resource
}

// Hints returns menu hints for this browser.
func (b *Browser) Hints() ui.MenuHints {
	return b.Actions().Hints()
}

// bindKeys sets up browser-specific key bindings.
func (b *Browser) bindKeys(aa *ui.KeyActions) {
	aa.Bulk(ui.KeyMap{
		ui.KeyR:        ui.NewKeyAction("Change Region", b.changeRegion, true),
		tcell.KeyCtrlR: ui.NewKeyAction("Refresh", b.refresh, true),
		ui.KeyD:        ui.NewKeyAction("Describe", b.describe, true),
		ui.KeyE:        ui.NewKeyAction("Edit", b.edit, true),
	})

	// Add action registry bindings for this resource type
	b.bindResourceActions(aa)
}

// bindResourceActions adds dynamic key bindings from the action registry.
func (b *Browser) bindResourceActions(aa *ui.KeyActions) {
	rid := b.GetResourceID()
	if rid == nil {
		return
	}

	actions := ui.GetActions(rid)
	for _, action := range actions {
		// Capture action in closure
		act := action
		handler := func(evt *tcell.EventKey) *tcell.EventKey {
			return b.executeAction(&act)
		}
		aa.Add(act.Key, ui.NewKeyAction(act.Name, handler, true))
	}
}

// executeAction executes a registered action, with confirmation for dangerous ones.
func (b *Browser) executeAction(action *ui.ResourceAction) *tcell.EventKey {
	resourceID := b.GetSelectedItem()
	if resourceID == "" {
		return nil
	}

	b.mx.RLock()
	app := b.app
	factory := b.factory
	region := b.region
	b.mx.RUnlock()

	if app == nil || factory == nil {
		return nil
	}

	client := factory.Client()
	if client == nil {
		app.Flash().Err(fmt.Errorf("failed to get AWS client"))
		return nil
	}

	// Get region from model if available
	if model := b.GetModel(); model != nil {
		if ns := model.GetNamespace(); ns != "" && ns != "*" && ns != "all" {
			region = ns
		}
	}
	if region == "" {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// If dangerous, show confirmation dialog
	if action.Dangerous {
		b.confirmAction(action, resourceID, region, client)
		return nil
	}

	// Execute action directly
	b.doExecuteAction(action, resourceID, region, client)
	return nil
}

// confirmAction shows a confirmation dialog for dangerous actions.
func (b *Browser) confirmAction(action *ui.ResourceAction, resourceID, region string, client aws.Connection) {
	b.mx.RLock()
	app := b.app
	b.mx.RUnlock()

	if app == nil {
		return
	}

	// Create and show confirmation dialog
	confirm := ui.NewConfirm(app.Content)
	confirm.SetMessage(fmt.Sprintf("%s %s?", action.Name, resourceID))
	confirm.SetDangerous(true)
	confirm.SetOnConfirm(func() {
		b.doExecuteAction(action, resourceID, region, client)
	})
	confirm.Show()
}

// doExecuteAction performs the actual action execution.
func (b *Browser) doExecuteAction(action *ui.ResourceAction, resourceID, region string, client aws.Connection) {
	b.mx.RLock()
	app := b.app
	b.mx.RUnlock()

	if app == nil {
		return
	}

	app.Flash().Infof("%s %s...", action.Name, resourceID)

	// Execute in goroutine to not block UI
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		err := action.Handler(ctx, client, region, resourceID)

		// Update UI on main thread
		app.QueueUpdateDraw(func() {
			if err != nil {
				app.Flash().Errf("%s failed: %v", action.Name, err)
			} else {
				app.Flash().Infof("%s %s successful", action.Name, resourceID)
				// Refresh the view
				b.refresh(nil)
			}
		})
	}()
}

// prepareContext creates a context with cancellation for data fetching.
func (b *Browser) prepareContext() context.Context {
	ctx := b.defaultContext()

	b.mx.Lock()
	if b.cancelFn != nil {
		b.cancelFn()
	}
	ctx, b.cancelFn = context.WithCancel(ctx)
	b.mx.Unlock()

	return ctx
}

// defaultContext builds the default context with resource ID and region.
func (b *Browser) defaultContext() context.Context {
	ctx := context.Background()

	if rid := b.GetResourceID(); rid != nil {
		ctx = context.WithValue(ctx, KeyResourceID, rid)
	}

	b.mx.RLock()
	region := b.region
	b.mx.RUnlock()

	ctx = context.WithValue(ctx, KeyRegion, region)

	return ctx
}

// refresh forces a data refresh.
func (b *Browser) refresh(*tcell.EventKey) *tcell.EventKey {
	b.Start()
	return nil
}

// describe shows resource details.
func (b *Browser) describe(*tcell.EventKey) *tcell.EventKey {
	resourceID := b.GetSelectedItem()
	if resourceID == "" {
		return nil
	}

	b.mx.RLock()
	pushFn := b.pushFn
	popFn := b.popFn
	factory := b.factory
	region := b.region
	app := b.app
	b.mx.RUnlock()

	if pushFn == nil {
		return nil
	}

	rid := b.GetResourceID()
	if rid == nil {
		return nil
	}

	// Get region for path - regional resources need region/id format
	if region == "" && factory != nil {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Build path based on resource type
	// Regional resources (EC2, VPC, EKS) use region/id format
	// Global resources (IAM, S3) just use the id/name
	path := resourceID
	if rid.Service == "ec2" || rid.Service == "vpc" || rid.Service == "eks" {
		path = region + "/" + resourceID
	}

	// Create describe view
	descView := NewDescribe(rid)
	descView.SetFactory(factory)
	descView.SetPath(path)
	descView.SetApp(app)
	descView.SetBackFn(func() {
		if popFn != nil {
			popFn()
		}
	})

	ctx := context.Background()
	if err := descView.Init(ctx); err != nil {
		return nil
	}

	// Push describe view onto stack
	pushFn("describe", descView)
	descView.Start()

	return nil
}

// edit opens the resource for editing via Cloud Control API.
func (b *Browser) edit(*tcell.EventKey) *tcell.EventKey {
	resourceID := b.GetSelectedItem()
	if resourceID == "" {
		return nil
	}

	rid := b.GetResourceID()
	if rid == nil {
		return nil
	}

	b.mx.RLock()
	app := b.app
	factory := b.factory
	region := b.region
	b.mx.RUnlock()

	// Check if we have app access for flash messages
	if app == nil {
		return nil
	}

	// Check if resource type is supported by Cloud Control
	_, ok := dao.GetCloudFormationType(rid)
	if !ok {
		app.Flash().Errf("Edit not supported for %s", rid.String())
		return nil
	}

	if factory == nil {
		app.Flash().Err(fmt.Errorf("factory not initialized"))
		return nil
	}

	client := factory.Client()
	if client == nil {
		app.Flash().Err(fmt.Errorf("failed to get AWS client"))
		return nil
	}

	// Get region - prefer the model's namespace (actual region of displayed data)
	if model := b.GetModel(); model != nil {
		if ns := model.GetNamespace(); ns != "" && ns != "*" && ns != "all" {
			region = ns
		}
	}
	if region == "" {
		region = factory.Region()
	}
	if region == "" {
		region = aws.DefaultRegion
	}

	// Build path (same logic as describe)
	path := resourceID
	if rid.Service == "ec2" || rid.Service == "vpc" || rid.Service == "eks" {
		path = region + "/" + resourceID
	}

	// Show info flash
	app.Flash().Infof("Opening editor for %s...", resourceID)

	// Call EditResource from editor module
	ctx := context.Background()
	err := EditResource(ctx, app.Application, client, rid, path, region)

	if err != nil {
		if err == ErrEditorCancelled {
			app.Flash().Info("Edit cancelled")
		} else if err == ErrNoChanges {
			app.Flash().Info("No changes detected")
		} else {
			app.Flash().Errf("Edit failed: %v", err)
		}
		return nil
	}

	// Success
	app.Flash().Infof("Successfully updated %s", resourceID)

	// Refresh the view to show updated data
	b.refresh(nil)

	return nil
}

// changeRegion prompts for region change.
func (b *Browser) changeRegion(*tcell.EventKey) *tcell.EventKey {
	// TODO: Implement region picker dialog
	return nil
}

// TableNoData notifies view no data is available.
func (b *Browser) TableNoData(mdata *model1.TableData) {
	b.mx.RLock()
	cancel := b.cancelFn
	b.mx.RUnlock()

	if cancel == nil {
		return
	}

	b.UpdateUI(mdata)
}

// TableDataChanged notifies view new data is available.
func (b *Browser) TableDataChanged(mdata *model1.TableData) {
	b.mx.RLock()
	cancel := b.cancelFn
	b.mx.RUnlock()

	if cancel == nil {
		return
	}

	b.UpdateUI(mdata)
}

// TableLoadFailed notifies view something went wrong.
func (b *Browser) TableLoadFailed(err error) {
	// TODO: Show error via App.Flash() when available
}

// friendlyError converts AWS errors to user-friendly messages.
func (b *Browser) friendlyError(err error, rid *dao.ResourceID) string {
	errStr := err.Error()

	// Check for common permission/access errors
	if strings.Contains(errStr, "AccessDenied") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "UnauthorizedOperation") ||
		strings.Contains(errStr, "not authorized") {
		return fmt.Sprintf("Access denied for %s", rid.String())
	}

	// Check for credential errors
	if strings.Contains(errStr, "NoCredentialProviders") ||
		strings.Contains(errStr, "ExpiredToken") ||
		strings.Contains(errStr, "InvalidClientTokenId") {
		return "AWS credentials invalid or expired"
	}

	// Check for region errors
	if strings.Contains(errStr, "InvalidRegion") ||
		strings.Contains(errStr, "endpoint") {
		return fmt.Sprintf("Invalid region for %s", rid.String())
	}

	// Check for network errors
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") {
		return "Unable to connect to AWS"
	}

	// Generic error - keep it short
	return fmt.Sprintf("Unable to list %s", rid.String())
}

// cleanseRegion normalizes region strings.
func cleanseRegion(region string) string {
	if region == "" || region == "all" || region == "*" {
		return aws.RegionAll
	}
	return region
}
