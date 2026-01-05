package model

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/a1s/a1s/internal/render"
)

// TableData fetches and manages resource data from a DAO.
type TableData struct {
	rid         *dao.ResourceID
	accessor    dao.Accessor
	factory     dao.Factory
	renderer    model1.Renderer
	region      string
	data        *model1.TableData
	refreshRate time.Duration
	listeners   []TableListener
	cancelFn    context.CancelFunc
	mx          sync.RWMutex
}

// NewTableData creates a new table data model.
func NewTableData(rid *dao.ResourceID, factory dao.Factory, refreshRate time.Duration) *TableData {
	return &TableData{
		rid:         rid,
		factory:     factory,
		data:        model1.NewTableData(),
		refreshRate: refreshRate,
		listeners:   make([]TableListener, 0, 2),
	}
}

// SetAccessor sets the DAO accessor.
func (t *TableData) SetAccessor(a dao.Accessor) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.accessor = a
}

// SetRenderer sets the renderer for converting DAO objects to rows.
func (t *TableData) SetRenderer(r model1.Renderer) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.renderer = r
}

// SetRegion sets the region filter.
func (t *TableData) SetRegion(region string) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.region = region
}

// Header returns the table header.
func (t *TableData) Header() model1.Header {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.data.Header()
}

// RowCount returns the number of rows.
func (t *TableData) RowCount() int {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.data.RowCount()
}

// RowEvents returns the current row events.
func (t *TableData) RowEvents() *model1.RowEvents {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.data.RowEvents()
}

// Empty returns true if no data is available.
func (t *TableData) Empty() bool {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.data.Empty()
}

// Peek returns a clone of the current table data.
func (t *TableData) Peek() *model1.TableData {
	t.mx.RLock()
	defer t.mx.RUnlock()
	return t.data.Clone()
}

// AddListener registers a table listener.
func (t *TableData) AddListener(l TableListener) {
	t.mx.Lock()
	defer t.mx.Unlock()
	t.listeners = append(t.listeners, l)
}

// RemoveListener unregisters a table listener.
func (t *TableData) RemoveListener(l TableListener) {
	t.mx.Lock()
	defer t.mx.Unlock()

	for i, listener := range t.listeners {
		if listener == l {
			t.listeners = append(t.listeners[:i], t.listeners[i+1:]...)
			return
		}
	}
}

// Watch starts watching/refreshing data periodically.
func (t *TableData) Watch(ctx context.Context) error {
	// Cancel any existing watch
	t.mx.Lock()
	if t.cancelFn != nil {
		t.cancelFn()
	}
	watchCtx, cancel := context.WithCancel(ctx)
	t.cancelFn = cancel
	t.mx.Unlock()

	// Initial fetch
	if err := t.Refresh(watchCtx); err != nil {
		t.notifyLoadFailed(err)
		return err
	}

	// Start periodic refresh
	go t.watchLoop(watchCtx)
	return nil
}

// watchLoop periodically refreshes data.
func (t *TableData) watchLoop(ctx context.Context) {
	t.mx.RLock()
	refreshRate := t.refreshRate
	t.mx.RUnlock()

	if refreshRate <= 0 {
		refreshRate = 5 * time.Second // Default refresh rate
	}

	ticker := time.NewTicker(refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := t.Refresh(ctx); err != nil {
				t.notifyLoadFailed(err)
			}
		}
	}
}

// Refresh fetches data from DAO immediately.
func (t *TableData) Refresh(ctx context.Context) error {
	t.mx.RLock()
	accessor := t.accessor
	renderer := t.renderer
	region := t.region
	t.mx.RUnlock()

	if accessor == nil {
		return fmt.Errorf("no accessor configured")
	}

	if renderer == nil {
		return fmt.Errorf("no renderer configured")
	}

	// Fetch data from DAO
	objects, err := accessor.List(ctx, region)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	// Convert to table data
	newData := model1.NewTableData()

	// Set header from renderer
	header := renderer.Header(region)
	newData.SetHeader(header)

	// Convert each object to a row
	rowEvents := model1.NewRowEvents(len(objects))
	for _, obj := range objects {
		row := model1.NewRow(len(header))
		if err := renderer.Render(obj, region, &row); err != nil {
			// Log error but continue with other rows
			continue
		}

		re := model1.NewRowEvent(model1.EventAdd, row)
		rowEvents.Add(re)
	}

	// Update internal data
	t.mx.Lock()
	oldEmpty := t.data.Empty()
	t.data = newData
	t.mx.Unlock()

	// Notify listeners
	if rowEvents.Empty() && !oldEmpty {
		t.notifyNoData(newData)
	} else {
		t.notifyDataChanged(newData)
	}

	return nil
}

// Stop stops the watch loop.
func (t *TableData) Stop() {
	t.mx.Lock()
	defer t.mx.Unlock()

	if t.cancelFn != nil {
		t.cancelFn()
		t.cancelFn = nil
	}
}

// notifyNoData notifies listeners that no data is available.
func (t *TableData) notifyNoData(data *model1.TableData) {
	t.mx.RLock()
	listeners := make([]TableListener, len(t.listeners))
	copy(listeners, t.listeners)
	t.mx.RUnlock()

	for _, l := range listeners {
		l.TableNoData(data)
	}
}

// notifyDataChanged notifies listeners that data has changed.
func (t *TableData) notifyDataChanged(data *model1.TableData) {
	t.mx.RLock()
	listeners := make([]TableListener, len(t.listeners))
	copy(listeners, t.listeners)
	t.mx.RUnlock()

	for _, l := range listeners {
		l.TableDataChanged(data)
	}
}

// notifyLoadFailed notifies listeners that loading failed.
func (t *TableData) notifyLoadFailed(err error) {
	t.mx.RLock()
	listeners := make([]TableListener, len(t.listeners))
	copy(listeners, t.listeners)
	t.mx.RUnlock()

	for _, l := range listeners {
		l.TableLoadFailed(err)
	}
}

// RendererFor returns the appropriate renderer for the given resource ID.
func RendererFor(rid *dao.ResourceID) (model1.Renderer, error) {
	switch rid.String() {
	case "ec2/instance":
		return &render.EC2Instance{}, nil
	case "ec2/volume":
		return &render.EC2Volume{}, nil
	case "ec2/securitygroup":
		return &render.SecurityGroup{}, nil
	case "vpc/vpc":
		return &render.VPC{}, nil
	case "vpc/subnet":
		return &render.Subnet{}, nil
	case "s3/bucket":
		return &render.S3Bucket{}, nil
	case "s3/object":
		return &render.S3Object{}, nil
	case "iam/user":
		return &render.IAMUser{}, nil
	case "iam/role":
		return &render.IAMRole{}, nil
	case "iam/policy":
		return &render.IAMPolicy{}, nil
	case "eks/cluster":
		return &render.EKSCluster{}, nil
	case "eks/nodegroup":
		return &render.EKSNodeGroup{}, nil
	default:
		return nil, fmt.Errorf("no renderer for resource: %s", rid.String())
	}
}
