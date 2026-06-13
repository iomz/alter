# alter

A local/private tool control plane for managed CLI plugins and MCP exposure.

`alter` is not a business-logic tool. It discovers and inspects plugin adapters. Actual tools such as `ingest` or `suuntool` do not need to know anything about `alter`, MCP, manifests, or schemas. Adapter plugins translate those tools into the alter contract.

No daemon runs. Long-running MCP mode is `alter mcp`.

## Architecture

- `alter`: CLI entrypoint built on `urfave/cli/v3`, plugin discovery, inspection, runtime discovery, MCP server mode
- `internal/runtime`: runtime discovery and execution boundary
- `internal/plugin`: typed plugin manifest parsing, discovery, inspection, and static doctor checks
- `mise`: plugin-local runtime manager
- `alter-foo`: adapter owned by `plugins/foo`
- `foo`: actual external tool wrapped by adapter

Plugin path is the local command name:

```text
plugins/hello
plugins/ingest
plugins/suuntool
```

Ownership and upstream information belong in `alter.plugin.toml`, not in filesystem path.

## Plugin Manifest

Each plugin directory contains `alter.plugin.toml`:

```toml
[plugin]
name = "hello"
description = "Example alter plugin"
maintainer = "iomz"
entrypoint = "alter-hello"

[upstream]
name = "hello"
repository = ""

[runtime]
manager = "mise"

[mcp]
enabled = true
namespace = "hello"
```

Manifests are parsed with `pelletier/go-toml` into strongly typed structures. Required
fields are:

- `plugin.name`
- `plugin.description`
- `plugin.entrypoint`
- `runtime.manager`

`plugin.name` must match the directory name. `runtime.manager` is currently `mise`.

## Plugin Layout

Prototype plugin directories:

```text
plugins/hello/
  alter.plugin.toml
  mise.toml
  alter-hello
  cmd/alter-hello/main.go
```

Foundation-only plugin directories may contain only a manifest:

```text
plugins/ingest/
  alter.plugin.toml

plugins/suuntool/
  alter.plugin.toml
```

## Commands

```sh
go run ./cmd/alter setup mise
go run ./cmd/alter setup shell
go run ./cmd/alter plugin list
go run ./cmd/alter plugin inspect hello
go run ./cmd/alter plugin doctor hello
```

`alter plugin doctor <name>` performs static checks only. It validates the manifest and
reports missing adapter entrypoints as warnings. It does not run plugins.

## Runtime Behavior

`alter` does not modify global shell config and does not require mise shell activation.

Runtime discovery is handled through a `MiseResolver` abstraction. It returns absolute
paths only and checks:

1. `mise` on `PATH`
2. `~/.local/share/alter/bin/mise`
3. `~/.local/bin/mise`

If mise is missing, `alter setup mise` explains the bootstrap plan and asks for
confirmation before installing anything.

For current prototype runtime preparation, `alter`:

1. discovers mise through the resolver
2. keeps shell activation optional
3. uses full paths internally
4. shows an actionable error if `mise` is missing

Prototype intentionally does not auto-trust arbitrary mise configs silently.

## Responsibility Boundaries

The alter core owns plugin discovery, manifest parsing, runtime discovery, and static
inspection. Adapter plugins own translation into upstream tools. Upstream tools do not
implement alter-specific interfaces.

Plugin execution and generated MCP exposure are future phases. They should remain outside
manifest parsing and static discovery logic.

## Setup

`alter setup mise` checks `PATH` first, then alter-managed locations. If mise is
still unavailable, it shows an interactive confirmation prompt before bootstrap.

When confirmed, alter:

1. downloads the official installer from `https://mise.run`
2. runs it with `MISE_INSTALL_PATH=~/.local/share/alter/bin/mise`
3. verifies the installed binary is executable
4. uses the full absolute path internally

`alter setup mise` never:

- modifies shell startup files
- runs `sudo`
- installs without confirmation
- configures future shell activation

`alter setup shell` is a styled stub. Shell integration remains optional and explicit;
alter does not modify shell startup files.

Terminal output uses Charmbracelet libraries:

- `lipgloss` for styled labels
- `glamour` for Markdown-rendered setup notes
- `huh` as the prompt styling foundation for future interactive setup
