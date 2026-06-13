# alter

A local/private tool control plane for managed CLI plugins and MCP exposure.

`alter` is not a business-logic tool. It discovers, checks, executes, and exposes plugin adapters. Actual tools such as `ingest` or `suuntool` do not need to know anything about `alter`, MCP, manifests, or schemas. Adapter plugins translate those tools into the alter contract.

No daemon runs. Long-running MCP mode is `alter mcp`.

## Architecture

- `alter`: CLI entrypoint, plugin discovery, inspection, execution wrapper, mise runtime resolver, MCP server mode
- `mise`: plugin-local runtime installation and command execution
- `alter-foo`: adapter owned by `plugins/foo`
- `foo`: actual external tool wrapped by adapter

Plugin path is the local command name:

```text
plugins/hello
plugins/ingest
plugins/suuntool
```

Ownership and upstream information belong in `alter.plugin.toml`, not in filesystem path.

## Plugin Contract

Each adapter supports:

```text
<entrypoint> manifest
<entrypoint> doctor
<entrypoint> invoke <json>
```

Prototype plugin:

```text
plugins/hello/
  alter.plugin.toml
  mise.toml
  alter-hello
  cmd/alter-hello/main.go
```

`alter-hello` returns predictable JSON for `greet`.

## Commands

```sh
go run ./cmd/alter plugin list
go run ./cmd/alter plugin inspect hello
go run ./cmd/alter plugin doctor hello
go run ./cmd/alter hello greet --name iomz
go run ./cmd/alter mcp
```

## Runtime Behavior

`alter` does not modify global shell config and does not require mise shell activation.

For plugin execution, `alter`:

1. runs commands from plugin workspace
2. uses `mise install` when preparing plugin through `plugin doctor`
3. uses `mise exec -- <entrypoint> ...` for adapter execution
4. prints a clear warning when plugin workspace contains `mise.toml`
5. shows an actionable error if `mise` is missing

Prototype intentionally does not auto-trust arbitrary mise configs silently.

## MCP

`alter mcp` exposes plugin commands as MCP tools over stdio.

Prototype exposes:

```text
hello_greet
```

MCP code is structured so future tools can be generated from adapter metadata.
