// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of a1s

package view

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/a1s/a1s/internal/aws"
	"github.com/a1s/a1s/internal/dao"
	"github.com/derailed/tview"
	"github.com/wI2L/jsondiff"
)

// Editor errors
var (
	ErrEditorCancelled = errors.New("editor cancelled")
	ErrNoChanges       = errors.New("no changes detected")
)

// EditSession represents an in-progress edit operation.
type EditSession struct {
	ResourceID   *dao.ResourceID
	TypeName     string   // CloudFormation type name
	Identifier   string   // Resource identifier
	Region       string
	OriginalJSON map[string]interface{} // Full resource state (for reference)
	EditableJSON map[string]interface{} // Filtered editable properties only
	Schema       *aws.ResourceSchema    // Schema with property classifications
	TempFile     string
	ErrorMsg     string // Error to display at top of file on retry
}

// NewEditSession creates a new edit session.
func NewEditSession(rid *dao.ResourceID, typeName, identifier, region string) *EditSession {
	return &EditSession{
		ResourceID: rid,
		TypeName:   typeName,
		Identifier: identifier,
		Region:     region,
	}
}

// FetchResource fetches the resource state via Cloud Control API.
func (e *EditSession) FetchResource(ctx context.Context, client aws.Connection) error {
	ccClient := client.CloudControl(e.Region)
	if ccClient == nil {
		return errors.New("failed to get CloudControl client")
	}

	props, err := aws.GetResourceState(ctx, ccClient, e.TypeName, e.Identifier)
	if err != nil {
		return err
	}

	e.OriginalJSON = props
	return nil
}

// StartEdit creates a temp file, spawns the editor, and returns the modified JSON.
// It suspends the TUI during editing.
func (e *EditSession) StartEdit(app *tview.Application) (map[string]interface{}, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "a1s-edit-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	e.TempFile = tmpFile.Name()

	// Write JSON content (with optional error message as comment)
	if err := e.writeJSONWithError(tmpFile); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	// Spawn editor (suspended TUI)
	exitCode, err := e.spawnEditor(app)
	if err != nil {
		return nil, fmt.Errorf("editor failed: %w", err)
	}

	// Check exit code - non-zero means cancel
	if exitCode != 0 {
		return nil, ErrEditorCancelled
	}

	// Read modified file
	content, err := os.ReadFile(e.TempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read edited file: %w", err)
	}

	// Strip any error comment block from top
	content = stripErrorComment(content)

	// Parse JSON
	var modified map[string]interface{}
	if err := json.Unmarshal(content, &modified); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	return modified, nil
}

// spawnEditor suspends the TUI and launches the editor.
func (e *EditSession) spawnEditor(app *tview.Application) (int, error) {
	editor := getEditor()

	var exitCode int
	suspended := app.Suspend(func() {
		cmd := exec.Command(editor, e.TempFile)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
	})

	if !suspended {
		return 1, errors.New("failed to suspend application")
	}

	return exitCode, nil
}

// writeJSONWithError writes JSON to the temp file, optionally with error at top.
func (e *EditSession) writeJSONWithError(f *os.File) error {
	var buf bytes.Buffer

	// If there's an error message, prepend it as a comment block
	if e.ErrorMsg != "" {
		buf.WriteString("// ERROR: " + e.ErrorMsg + "\n")
		buf.WriteString("// Fix the issue below and save, or save without changes to cancel.\n")
		buf.WriteString("// ---\n\n")
	}

	// Use EditableJSON if available, otherwise fall back to OriginalJSON
	jsonToWrite := e.EditableJSON
	if jsonToWrite == nil {
		jsonToWrite = e.OriginalJSON
	}

	// Pretty-print the JSON
	jsonBytes, err := json.MarshalIndent(jsonToWrite, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	buf.Write(jsonBytes)
	buf.WriteString("\n")

	_, err = f.Write(buf.Bytes())
	return err
}

// GeneratePatch creates a JSON Patch document from original and modified JSON.
// Returns the patch as a JSON string, or ErrNoChanges if identical.
func GeneratePatch(original, modified map[string]interface{}) (string, error) {
	// Use jsondiff to generate RFC 6902 patch
	patch, err := jsondiff.Compare(original, modified)
	if err != nil {
		return "", fmt.Errorf("failed to generate patch: %w", err)
	}

	if len(patch) == 0 {
		return "", ErrNoChanges
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("failed to marshal patch: %w", err)
	}

	return string(patchBytes), nil
}

// ApplyUpdate sends the patch to Cloud Control API.
func (e *EditSession) ApplyUpdate(ctx context.Context, client aws.Connection, patchDocument string) error {
	ccClient := client.CloudControl(e.Region)
	if ccClient == nil {
		return errors.New("failed to get CloudControl client")
	}

	return aws.UpdateResourceState(ctx, ccClient, e.TypeName, e.Identifier, patchDocument)
}

// Cleanup removes the temporary file.
func (e *EditSession) Cleanup() {
	if e.TempFile != "" {
		os.Remove(e.TempFile)
		e.TempFile = ""
	}
}

// SetError sets the error message for display on retry.
func (e *EditSession) SetError(msg string) {
	e.ErrorMsg = msg
}

// getEditor returns the editor command to use.
// Checks $EDITOR, then falls back to vim, then nano.
func getEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	// Check if vim exists
	if _, err := exec.LookPath("vim"); err == nil {
		return "vim"
	}
	// Fall back to nano
	return "nano"
}

// stripErrorComment removes the error comment block from the top of content.
func stripErrorComment(content []byte) []byte {
	lines := bytes.Split(content, []byte("\n"))
	startIdx := 0

	for i, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		if bytes.HasPrefix(trimmed, []byte("//")) {
			startIdx = i + 1
			continue
		}
		// Found non-comment, non-empty line
		break
	}

	if startIdx > 0 && startIdx < len(lines) {
		return bytes.Join(lines[startIdx:], []byte("\n"))
	}
	return content
}

// EditResource performs the full edit flow for a resource.
// This is the main entry point for the edit feature.
func EditResource(ctx context.Context, app *tview.Application, client aws.Connection, rid *dao.ResourceID, path, region string) error {
	// Get CloudFormation type
	typeName, ok := dao.GetCloudFormationType(rid)
	if !ok {
		return fmt.Errorf("edit not supported for %s", rid.String())
	}

	// Extract identifier from path
	identifier := aws.ExtractIdentifier(rid.String(), path)

	// Create session
	session := NewEditSession(rid, typeName, identifier, region)
	defer session.Cleanup()

	// Fetch current state
	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := session.FetchResource(fetchCtx, client); err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}

	// Fetch schema and filter to editable properties only
	cfClient := client.CloudFormation(region)
	if cfClient != nil {
		schemaCtx, schemaCancel := context.WithTimeout(ctx, 10*time.Second)
		schema, err := aws.GetResourceSchema(schemaCtx, cfClient, typeName)
		schemaCancel()

		if err == nil && schema != nil {
			session.Schema = schema
			session.EditableJSON = aws.FilterEditableProperties(session.OriginalJSON, schema)
		}
	}

	// If no schema available, fall back to showing all properties
	if session.EditableJSON == nil {
		session.EditableJSON = session.OriginalJSON
	}

	// Edit loop (allows retry on error)
	for {
		// Open editor
		modified, err := session.StartEdit(app)
		if err != nil {
			if errors.Is(err, ErrEditorCancelled) {
				return ErrEditorCancelled
			}
			return err
		}

		// Generate patch (compare against editable properties)
		patch, err := GeneratePatch(session.EditableJSON, modified)
		if err != nil {
			if errors.Is(err, ErrNoChanges) {
				// If we had an error message and user saved without changes, treat as cancel
				if session.ErrorMsg != "" {
					return ErrEditorCancelled
				}
				return ErrNoChanges
			}
			return err
		}

		// Apply update
		updateCtx, updateCancel := context.WithTimeout(ctx, 2*time.Minute)
		err = session.ApplyUpdate(updateCtx, client, patch)
		updateCancel()

		if err != nil {
			// Set error and retry
			session.SetError(err.Error())
			// Update editable to match what user submitted (for next diff)
			session.EditableJSON = modified
			continue
		}

		// Success
		return nil
	}
}
