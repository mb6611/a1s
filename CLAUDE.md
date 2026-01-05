# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

a1s is a terminal-based UI for managing AWS resources, inspired by k9s (the Kubernetes CLI). It uses tview for the TUI and AWS SDK v2 for AWS interactions.

## Build & Run Commands

```bash
make build          # Build to bin/a1s
make run            # Build and run
make clean          # Remove build artifacts
make test           # Run tests
make fmt            # Format code
make lint           # Run golangci-lint
make build-all      # Cross-compile for all platforms

# Run with specific profile/region
./bin/a1s --profile myprofile --region us-west-2
```

## Architecture

### Layer Overview

```
cmd/root.go          → Entry point, CLI flags, initialization
    ↓
internal/view/       → TUI views (App, Browser, Table, Help)
    ↓
internal/ui/         → Reusable UI components (CmdBar, Dialog, Menu)
    ↓
internal/dao/        → Data Access Objects for AWS resources
    ↓
internal/aws/        → AWS SDK client wrapper and helpers
    ↓
internal/config/     → Configuration management
```

### Key Concepts

**ResourceID**: Replaces Kubernetes GVR (Group/Version/Resource). Defined as `{Service, Resource}` e.g., `{ec2, instance}`, `{s3, bucket}`.

**Accessor Pattern**: Each AWS resource type has a DAO implementing the `Accessor` interface:
- `Init(Factory, *ResourceID)` - Initialize with factory
- `List(ctx, region)` - List resources
- `Get(ctx, path)` - Get single resource
- DAOs register themselves via `init()` with `RegisterAccessor()`

**Action Registry**: Resource-specific actions (start/stop/delete) are registered in `ui/action_registry.go`:
```go
RegisterActions("ec2/instance", []ResourceAction{
    {Key: KeyS, Name: "Stop", Dangerous: true, Handler: ...},
})
```

**Browser Pattern**: `view/Browser` is the base for all resource list views. Resource-specific views (EC2Instance, S3Browser) embed Browser and add custom keybindings.

### Important Files

- `internal/aws/client.go`: `Connection` interface and `APIClient` - all AWS SDK clients
- `internal/dao/types.go`: Core interfaces (`AWSObject`, `Accessor`, `Factory`, `ResourceID`)
- `internal/dao/accessor.go`: Global accessor registry and `AccessorFor()` factory
- `internal/view/app.go`: Main application, layout, keyboard handling
- `internal/view/browser.go`: Base resource browser with table display
- `internal/ui/action_registry.go`: Resource action registration system

### Adding a New AWS Resource

1. Create DAO in `internal/dao/` (e.g., `lambda_function.go`):
   - Define `ResourceID` variable
   - Implement struct embedding `AWSResource`
   - Implement `List()`, `Get()`, `Describe()`
   - Register in `init()` with `RegisterAccessor()`

2. Optionally create custom view in `internal/view/` for special UI

3. Add command alias in `internal/view/command.go`

4. Register actions in `internal/ui/` if needed

### AWS Client Access

```go
// Get regional client
ec2Client := factory.Client().EC2(region)

// S3 is special - use S3Regional for bucket operations
s3Client := factory.Client().S3Regional(bucketRegion)

// Global services (IAM)
iamClient := factory.Client().IAM()
```

### TUI Patterns

- Views implement `ui.Component` interface
- Use `app.Flash().Infof()` for status messages
- Dangerous actions should set `Dangerous: true` to show confirmation dialog
- Use `app.Suspend()` to shell out (SSH, SSM sessions)
