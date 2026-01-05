# TableData Model Layer Usage

## Overview

The `TableData` model layer connects the Browser UI to DAO accessors for fetching real AWS data.

## Basic Usage

```go
package view

import (
    "time"
    "github.com/a1s/a1s/internal/dao"
    "github.com/a1s/a1s/internal/model"
)

func setupTableModel(factory dao.Factory, rid *dao.ResourceID) (*model.TableData, error) {
    // 1. Create the table data model with refresh rate
    tableData := model.NewTableData(rid, factory, 5*time.Second)

    // 2. Get the appropriate accessor from factory
    accessor, err := dao.AccessorFor(factory, rid)
    if err != nil {
        return nil, err
    }
    tableData.SetAccessor(accessor)

    // 3. Get the appropriate renderer for the resource type
    renderer, err := model.RendererFor(rid)
    if err != nil {
        return nil, err
    }
    tableData.SetRenderer(renderer)

    // 4. Set region filter (optional, defaults to all regions)
    tableData.SetRegion("us-west-2")

    return tableData, nil
}

// Example: Browser integration
func (b *Browser) setupModel(ctx context.Context) error {
    factory := getFactoryFromContext(ctx)
    rid := b.GetResourceID()

    tableData, err := setupTableModel(factory, rid)
    if err != nil {
        return err
    }

    // Register the browser as a listener
    tableData.AddListener(b)

    // Start watching (this will fetch data and start periodic refresh)
    return tableData.Watch(ctx)
}
```

## Integration with ui.Table

The `ui.Table` expects a `Tabular` interface. You can create an adapter:

```go
// In internal/ui/table_model.go
type TableModel struct {
    *model.TableData
    namespace string
    mx        sync.RWMutex
}

func NewTableModel(rid *dao.ResourceID, factory dao.Factory) *TableModel {
    return &TableModel{
        TableData: model.NewTableData(rid, factory, 5*time.Second),
    }
}

// Implement Namespaceable interface (for AWS, these are no-ops)
func (t *TableModel) ClusterWide() bool          { return true }
func (t *TableModel) GetNamespace() string       { return "" }
func (t *TableModel) SetNamespace(ns string)     {}
func (t *TableModel) InNamespace(ns string) bool { return true }

// Implement Lister interface
func (t *TableModel) Get(ctx context.Context, path string) (interface{}, error) {
    accessor := t.GetAccessor()
    if accessor == nil {
        return nil, fmt.Errorf("no accessor configured")
    }
    return accessor.Get(ctx, path)
}

// SetInstance sets parent resource path (for nested resources like S3 objects)
func (t *TableModel) SetInstance(path string) {
    // Can be used to set parent resource for nested browsing
}

// Empty returns true if model has no data
func (t *TableModel) Empty() bool {
    return t.TableData.Empty()
}

// Peek returns current model data
func (t *TableModel) Peek() *model1.TableData {
    return t.TableData.Peek()
}

// SetRefreshRate sets the model watch loop rate
func (t *TableModel) SetRefreshRate(d time.Duration) {
    // Would need to add this method to TableData
}
```

## Supported Resource Types

The `RendererFor` function supports all registered AWS resource types:

- EC2: `ec2/instance`, `ec2/volume`, `ec2/securitygroup`
- VPC: `vpc/vpc`, `vpc/subnet`
- S3: `s3/bucket`, `s3/object`
- IAM: `iam/user`, `iam/role`, `iam/policy`
- EKS: `eks/cluster`, `eks/nodegroup`

## Data Flow

```
1. Browser.Start()
   → TableData.Watch(ctx)
   → TableData.Refresh(ctx)
   → Accessor.List(ctx, region)
   → Returns []AWSObject

2. TableData converts data:
   → Renderer.Header(region) → creates model1.Header
   → For each AWSObject:
     → Renderer.Render(obj, region, row) → creates model1.Row
     → Wraps in model1.RowEvent
     → Adds to model1.RowEvents

3. TableData notifies listeners:
   → TableListener.TableDataChanged(*model1.TableData)
   → Browser.TableDataChanged(data)
   → Browser.UpdateUI(data)
   → ui.Table renders rows

4. Periodic refresh (every 5 seconds by default):
   → Repeats steps 1-3
```

## Stopping the Model

```go
// Call Stop() to cancel the watch loop and cleanup
tableData.Stop()
```
