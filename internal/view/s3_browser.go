// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/a1s/a1s/internal/dao"
	"github.com/a1s/a1s/internal/model1"
	"github.com/a1s/a1s/internal/ui"
	"github.com/derailed/tcell/v2"
)

// S3Browser represents an S3 bucket/object browser with hierarchical navigation.
type S3Browser struct {
	*Browser

	currentBucket string
	currentPrefix string
	breadcrumbs   []string
}

// NewS3Browser returns a new S3 browser.
func NewS3Browser() *S3Browser {
	rid := &dao.ResourceID{
		Service:  "s3",
		Resource: "bucket",
	}
	s := &S3Browser{
		Browser:     NewBrowser(rid),
		breadcrumbs: []string{},
	}

	return s
}

// Init initializes the S3 browser.
func (s *S3Browser) Init(ctx context.Context) error {
	if err := s.Browser.Init(ctx); err != nil {
		return err
	}

	s.bindS3Keys(s.Actions())
	return nil
}

// Name returns the current bucket/prefix name for breadcrumbs.
func (s *S3Browser) Name() string {
	if s.currentBucket == "" {
		return "S3 Buckets"
	}
	if s.currentPrefix == "" {
		return s.currentBucket
	}
	return s.currentBucket + "/" + s.currentPrefix
}

// Start starts the S3 browser with proper bucket/prefix context.
func (s *S3Browser) Start() {
	s.Stop()

	// If no bucket selected, show bucket list (use parent's logic)
	if s.currentBucket == "" {
		s.Browser.Start()
		return
	}

	// Load objects for the current bucket/prefix
	s.loadS3Objects()
}

// loadS3Objects fetches S3 objects for the current bucket/prefix.
func (s *S3Browser) loadS3Objects() {
	s.mx.RLock()
	factory := s.factory
	s.mx.RUnlock()

	if factory == nil {
		return
	}

	// Build the path for S3Object.List
	path := s.currentBucket
	if s.currentPrefix != "" {
		path = s.currentBucket + "/" + s.currentPrefix
	}

	// Get S3 object accessor
	rid := &dao.ResourceID{Service: "s3", Resource: "object"}
	accessor, err := dao.AccessorFor(factory, rid)
	if err != nil {
		s.showError("Failed to get S3 accessor")
		return
	}

	// Fetch objects
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	objects, err := accessor.List(ctx, path)
	if err != nil {
		s.showError(fmt.Sprintf("Failed to list objects: %v", err))
		return
	}

	// Render objects
	data := s.renderS3Objects(objects)
	s.UpdateUI(data)
}

// renderS3Objects converts S3 objects to TableData.
func (s *S3Browser) renderS3Objects(objects []dao.AWSObject) *model1.TableData {
	data := model1.NewTableData()
	data.SetNamespace(s.currentBucket)

	// Set header for S3 objects
	data.SetHeader(model1.Header{
		{Name: "NAME"},
		{Name: "SIZE"},
		{Name: "LAST MODIFIED"},
		{Name: "STORAGE CLASS"},
	})

	for _, obj := range objects {
		row := model1.NewRow(4)
		row.ID = obj.GetID()
		row.Fields[0] = obj.GetName()

		// Check if it's a folder
		if strings.HasSuffix(obj.GetName(), "/") {
			row.Fields[1] = "-"
			row.Fields[2] = "-"
			row.Fields[3] = "Folder"
		} else {
			// Default values
			row.Fields[1] = "-"
			row.Fields[3] = "STANDARD"

			if t := obj.GetCreatedAt(); t != nil {
				row.Fields[2] = t.Format("2006-01-02 15:04")
			} else {
				row.Fields[2] = "-"
			}
		}

		data.RowEvents().Add(model1.NewRowEvent(model1.EventAdd, row))
	}

	return data
}

// showError displays an error in the table.
func (s *S3Browser) showError(msg string) {
	data := model1.NewTableData()
	data.SetNamespace(s.currentBucket)
	data.SetError(msg)
	s.UpdateUI(data)
}

// formatBytes formats bytes to human-readable size.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// Stop stops the S3 browser.
func (s *S3Browser) Stop() {
	s.Browser.Stop()
}

// SetBucket sets the current bucket.
func (s *S3Browser) SetBucket(bucket string) {
	s.currentBucket = bucket
	if bucket != "" {
		s.breadcrumbs = []string{bucket}
	} else {
		s.breadcrumbs = []string{}
	}
}

// SetPrefix sets the current prefix within the bucket.
func (s *S3Browser) SetPrefix(prefix string) {
	s.currentPrefix = prefix
	// Update breadcrumbs based on prefix path segments
	if s.currentBucket != "" {
		s.breadcrumbs = []string{s.currentBucket}
		if prefix != "" {
			// Split prefix by "/" to build breadcrumbs
			segments := splitPrefix(prefix)
			s.breadcrumbs = append(s.breadcrumbs, segments...)
		}
	}
}

// splitPrefix splits a prefix into path segments.
func splitPrefix(prefix string) []string {
	if prefix == "" {
		return []string{}
	}
	parts := strings.Split(prefix, "/")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// bindS3Keys binds S3-specific key actions.
func (s *S3Browser) bindS3Keys(aa *ui.KeyActions) {
	aa.Bulk(ui.KeyMap{
		tcell.KeyEnter:     ui.NewKeyAction("Drill Down", s.drillDownCmd, true),
		tcell.KeyBackspace: ui.NewKeyAction("Go Up", s.goUpCmd, true),
		tcell.KeyEsc:       ui.NewKeyAction("Go Up", s.goUpCmd, false),
		ui.KeyD:            ui.NewKeyAction("Download", s.downloadCmd, true),
		ui.KeyU:            ui.NewKeyAction("Upload", s.uploadCmd, true),
		tcell.KeyCtrlD: ui.NewKeyActionWithOpts("Delete", s.deleteCmd, ui.ActionOpts{
			Visible:   true,
			Dangerous: true,
		}),
	})
}

// drillDownCmd handles drilling down into a bucket or prefix.
func (s *S3Browser) drillDownCmd(evt *tcell.EventKey) *tcell.EventKey {
	// Get selected item
	path := s.GetSelectedItem()
	if path == "" {
		return nil
	}

	// If at bucket list, enter the bucket
	if s.currentBucket == "" {
		s.SetBucket(path)
		s.Start()
		return nil
	}

	// If it's a "folder" (ends with /), enter it
	if strings.HasSuffix(path, "/") {
		s.SetPrefix(s.currentPrefix + path)
		s.Start()
		return nil
	}

	// It's a file - show details
	// TODO: Show object details view
	return nil
}

// goUpCmd handles going up one level in the hierarchy.
func (s *S3Browser) goUpCmd(evt *tcell.EventKey) *tcell.EventKey {
	if len(s.breadcrumbs) == 0 {
		return evt
	}

	if len(s.breadcrumbs) == 1 {
		// At bucket level, go back to bucket list
		s.currentBucket = ""
		s.currentPrefix = ""
		s.breadcrumbs = []string{}
	} else {
		// Remove last breadcrumb segment
		s.breadcrumbs = s.breadcrumbs[:len(s.breadcrumbs)-1]
		s.currentBucket = s.breadcrumbs[0]

		// Rebuild prefix from remaining breadcrumbs (excluding bucket)
		if len(s.breadcrumbs) > 1 {
			s.currentPrefix = strings.Join(s.breadcrumbs[1:], "/") + "/"
		} else {
			s.currentPrefix = ""
		}
	}

	s.Start()
	return nil
}

// downloadCmd handles downloading an S3 object.
func (s *S3Browser) downloadCmd(evt *tcell.EventKey) *tcell.EventKey {
	// Get selected item
	name := s.GetSelectedItem()
	if name == "" {
		return nil
	}

	// Must be inside a bucket to download
	if s.currentBucket == "" {
		return nil
	}

	// Can't download folders
	if strings.HasSuffix(name, "/") {
		s.mx.RLock()
		app := s.app
		s.mx.RUnlock()
		if app != nil {
			app.Flash().Warn("Cannot download folders. Navigate into the folder to download files.")
		}
		return nil
	}

	s.mx.RLock()
	app := s.app
	factory := s.factory
	s.mx.RUnlock()

	if app == nil || factory == nil {
		return nil
	}

	// Build the key path
	key := s.currentPrefix + name

	// Determine download location (use ~/Downloads if exists, else current dir)
	downloadDir := getDownloadDir()
	localPath := downloadDir + "/" + name

	app.Flash().Infof("Downloading %s to %s...", name, localPath)

	// Run download in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		err := s.doDownload(ctx, s.currentBucket, key, localPath)

		app.QueueUpdateDraw(func() {
			if err != nil {
				app.Flash().Errf("Download failed: %v", err)
			} else {
				app.Flash().Infof("Downloaded %s to %s", name, localPath)
			}
		})
	}()

	return nil
}

// doDownload performs the actual S3 download.
func (s *S3Browser) doDownload(ctx context.Context, bucket, key, localPath string) error {
	s.mx.RLock()
	factory := s.factory
	s.mx.RUnlock()

	if factory == nil {
		return fmt.Errorf("no factory available")
	}

	// Get S3 object accessor
	rid := &dao.ResourceID{Service: "s3", Resource: "object"}
	accessor, err := dao.AccessorFor(factory, rid)
	if err != nil {
		return fmt.Errorf("failed to get S3 accessor: %w", err)
	}

	// Type assert to get Download method
	downloader, ok := accessor.(interface {
		Download(ctx context.Context, bucket, key string, writer io.Writer) error
	})
	if !ok {
		return fmt.Errorf("S3 accessor does not support download")
	}

	// Create local file
	file, err := createFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Download
	return downloader.Download(ctx, bucket, key, file)
}

// getDownloadDir returns the download directory path.
func getDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	downloadsDir := home + "/Downloads"
	if info, err := os.Stat(downloadsDir); err == nil && info.IsDir() {
		return downloadsDir
	}

	return home
}

// createFile creates a file, handling existing files.
func createFile(path string) (*os.File, error) {
	return os.Create(path)
}

// uploadCmd handles uploading a file to S3.
func (s *S3Browser) uploadCmd(evt *tcell.EventKey) *tcell.EventKey {
	// TODO: Implement upload
	return nil
}

// deleteCmd handles deleting an S3 object.
func (s *S3Browser) deleteCmd(evt *tcell.EventKey) *tcell.EventKey {
	// Get selected item
	name := s.GetSelectedItem()
	if name == "" {
		return nil
	}

	// Must be inside a bucket to delete
	if s.currentBucket == "" {
		return nil
	}

	s.mx.RLock()
	app := s.app
	factory := s.factory
	s.mx.RUnlock()

	if app == nil || factory == nil {
		return nil
	}

	// Build full path for deletion
	fullPath := s.currentBucket + "/"
	if s.currentPrefix != "" {
		fullPath += s.currentPrefix
	}
	fullPath += name

	// Determine if it's a folder
	isFolder := strings.HasSuffix(name, "/")

	// Build confirmation message
	var msg string
	if isFolder {
		msg = fmt.Sprintf("Delete folder '%s' and ALL its contents?\n\nThis action cannot be undone!", name)
	} else {
		msg = fmt.Sprintf("Delete object '%s'?\n\nThis action cannot be undone!", name)
	}

	// Show confirmation dialog
	confirm := ui.NewConfirm(app.Content)
	confirm.SetMessage(msg)
	confirm.SetDangerous(true)
	confirm.SetOnConfirm(func() {
		s.doDelete(fullPath, isFolder)
	})
	confirm.Show()

	return nil
}

// doDelete performs the actual S3 deletion.
func (s *S3Browser) doDelete(path string, isFolder bool) {
	s.mx.RLock()
	app := s.app
	factory := s.factory
	s.mx.RUnlock()

	if app == nil || factory == nil {
		return
	}

	// Get S3 object accessor
	rid := &dao.ResourceID{Service: "s3", Resource: "object"}
	accessor, err := dao.AccessorFor(factory, rid)
	if err != nil {
		app.Flash().Errf("Failed to get S3 accessor: %v", err)
		return
	}

	// Type assert to get Delete method
	deleter, ok := accessor.(interface {
		Delete(ctx context.Context, path string, force bool) error
	})
	if !ok {
		app.Flash().Errf("S3 accessor does not support delete")
		return
	}

	app.Flash().Infof("Deleting %s...", path)

	// Run deletion in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// force=true for folders to delete all contents
		err := deleter.Delete(ctx, path, isFolder)

		app.QueueUpdateDraw(func() {
			if err != nil {
				app.Flash().Errf("Delete failed: %v", err)
			} else {
				app.Flash().Infof("Deleted %s", path)
				// Refresh the view
				s.Start()
			}
		})
	}()
}
