# alter

A local/private tool control plane for managed CLI plugins and MCP exposure.

`alter` is not a business-logic tool. It discovers and inspects plugin adapters. Actual tools such as `ingest` or `suuntool` do not need to know anything about `alter`, MCP, manifests, or schemas. Adapter plugins translate those tools into the alter contract.

No daemon runs. MCP mode is `alter mcp` over stdio.

## Architecture

- `alter`: CLI entrypoint built on `urfave/cli/v3`, plugin discovery, inspection, runtime discovery, adapter invocation, and MCP stdio serving
- `internal/runtime`: runtime discovery and execution boundary
- `internal/plugin`: typed plugin manifest parsing, discovery, inspection, and static layout checks
- `internal/adapter`: adapter execution and output normalization
- `internal/mcp`: MCP server setup, tool registration, and adapter-backed tool calls
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
  alter.mise.toml
  alter-hello
  cmd/alter-hello/main.go
```

## Adapter Contract

Executable adapters expose three commands:

```text
<entrypoint> manifest
<entrypoint> doctor
<entrypoint> invoke <json>
```

`invoke` receives a JSON envelope:

```json
{
  "tool": "greet",
  "args": {
    "name": "iomz"
  }
}
```

Adapters return JSON. alter validates and pretty-prints adapter JSON before writing it to
stdout.

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
go run ./cmd/alter setup cleanup
go run ./cmd/alter plugin list
go run ./cmd/alter plugin inspect hello
go run ./cmd/alter plugin doctor hello
go run ./cmd/alter hello greet --name iomz
go run ./cmd/alter mcp
```

`alter plugin doctor <name>` performs static layout checks first. If an adapter entrypoint
exists, it runs adapter `doctor` through the runtime wrapper. Manifest-only plugin
directories report missing entrypoints as warnings.

## Runtime Behavior

`alter` does not modify global shell config and does not require mise shell activation.

Runtime discovery is handled through a `MiseResolver` abstraction. It returns absolute
paths only and checks:

1. `mise` on `PATH`
2. `~/.local/share/alter/bin/mise`
3. `~/.local/bin/mise`

If mise is missing, `alter setup mise` explains the bootstrap plan and asks for
confirmation before installing anything.

For plugin execution, `alter`:

1. discovers mise through the resolver
2. runs `mise install` inside the plugin workspace
3. runs `mise exec -- <entrypoint> ...` inside the plugin workspace
4. validates adapter JSON output
5. uses full paths internally
6. shows an actionable error if `mise` is missing

Plugin runtime execution is isolated from user global mise/asdf configuration. alter sets:

```text
MISE_OVERRIDE_CONFIG_FILENAMES=alter.mise.toml
MISE_GLOBAL_CONFIG_FILE=~/.local/state/alter/mise/config.toml
MISE_DATA_DIR=~/.local/state/alter/mise/data
MISE_CACHE_DIR=~/.cache/alter/mise
```

Plugin runtime config lives in `alter.mise.toml`, not `mise.toml`, so mise does not read
`.tool-versions` or user global mise config during alter-managed plugin execution.

Prototype intentionally does not auto-trust arbitrary mise configs silently.

## Responsibility Boundaries

The alter core owns plugin discovery, manifest parsing, runtime discovery, runtime
wrapping, and adapter output normalization. Adapter plugins own translation into upstream
tools. Upstream tools do not implement alter-specific interfaces.

Execution flow:

```text
alter
-> plugin adapter contract
-> runtime wrapper
-> output normalization
```

Adapter internals may call upstream tools. That call remains adapter-owned.

Generated MCP exposure is future work. Current MCP registration is explicit and should
remain outside manifest parsing and static discovery logic.

## MCP

`alter mcp` serves MCP over stdio using `modelcontextprotocol/go-sdk`.

Current exposed tool:

```text
hello_greet
```

Tool registration is intentionally explicit. The current path is:

```text
plugin metadata
-> tool registration
-> MCP exposure
-> adapter invocation
```

`hello_greet` calls the `hello` adapter's `greet` tool and returns adapter JSON as both
text content and structured content.

Future direction:

- derive MCP tools from adapter metadata
- expose more plugin tools after adapter metadata stabilizes
- keep MCP registration separate from plugin manifest parsing
- keep transport-specific code thin

## Setup

`alter setup mise` checks `PATH` first, then alter-managed locations. If mise is
still unavailable, it shows an interactive confirmation prompt before bootstrap.

When confirmed, alter:

1. downloads the official installer from `https://mise.run`
2. runs it with `MISE_INSTALL_PATH=~/.local/share/alter/bin/mise`
3. captures installer stdout/stderr
4. shows alter-owned success output
5. prints raw installer output only if installation fails
6. verifies the installed binary is executable
7. uses the full absolute path internally

`alter setup mise` never:

- modifies shell startup files
- runs `sudo`
- installs without confirmation
- configures future shell activation

`alter setup cleanup` removes only alter-managed mise runtime files:

- `~/.local/share/alter/bin/mise`
- `~/.local/state/alter/mise`
- `~/.cache/alter/mise`

It never removes shell startup files, `~/.tool-versions`, user global mise config, or asdf
files.

`alter setup shell` is a styled stub. Shell integration remains optional and explicit;
alter does not modify shell startup files.

Terminal output uses Charmbracelet libraries:

- `lipgloss` for styled labels
- `glamour` for Markdown-rendered setup notes
- `huh` as the prompt styling foundation for future interactive setup
