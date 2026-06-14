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

plugins/test-runtime/
  alter.plugin.toml
  alter.mise.toml
  alter-test-runtime
```

`test-runtime` is a dedicated mise isolation proof plugin. It explicitly declares
only Node.js in `alter.mise.toml`:

```toml
[tools]
node = "24"
```

It exists to verify that mise mode installs only plugin-declared runtimes and ignores
user/global mise or asdf state before real plugins such as `ingest` are wired in.

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
go run ./cmd/alter plugin inspect hello --json
go run ./cmd/alter plugin doctor hello
go run ./cmd/alter plugin doctor test-runtime
go run ./cmd/alter plugin trust-status test-runtime
go run ./cmd/alter plugin trust test-runtime
go run ./cmd/alter hello greet --name iomz
ALTER_LOG=debug go run ./cmd/alter test-runtime node-version
go run ./cmd/alter plugin untrust test-runtime
go run ./cmd/alter mcp
```

Human-facing commands use structured sections, compact tables, and semantic status
styles. `alter plugin inspect <name>` defaults to a readable manifest summary; use
`--json` for raw parseable manifest output.

`alter plugin doctor <name>` performs static layout checks first. If an adapter entrypoint
exists, it prints runtime isolation diagnostics. Manifest-only plugin directories report
missing entrypoints as warnings.

## Runtime Behavior

`alter` does not modify global shell config and does not require mise shell activation.

Runtime discovery is handled through a `MiseResolver` abstraction. It returns absolute
paths only and checks:

1. `mise` on `PATH`
2. `~/.local/share/alter/bin/mise`
3. `~/.local/bin/mise`

If mise is missing, `alter setup mise` explains the bootstrap plan and asks for
confirmation before installing anything.

For plugin execution, `alter` first chooses a runtime mode:

- `direct`: run the adapter entrypoint directly from the plugin workspace
- `mise`: run through mise only when plugin runtime config declares tools

Direct mode is the default when `alter.mise.toml` is missing or has no `[tools]`
entries and `alter.tool-versions` is missing or empty. In direct mode, alter does
not call `mise install` and does not call `mise exec`.

In mise mode, `alter`:

1. discovers mise through the resolver
2. runs `mise install` inside the plugin workspace
3. runs `mise exec -- <entrypoint> ...` inside the plugin workspace
4. validates adapter JSON output
5. uses full paths internally
6. shows an actionable error if `mise` is missing

Plugin runtime execution is isolated from user global mise/asdf configuration. alter sets:

```text
MISE_OVERRIDE_CONFIG_FILENAMES=alter.mise.toml
MISE_OVERRIDE_TOOL_VERSIONS_FILENAME=alter.tool-versions
MISE_OVERRIDE_TOOL_VERSIONS_FILENAMES=alter.tool-versions
MISE_LEGACY_VERSION_FILE=false
MISE_ASDF_COMPAT=false
MISE_GLOBAL_CONFIG_FILE=~/.local/state/alter/mise/config.toml
MISE_DATA_DIR=~/.local/state/alter/mise/data
MISE_CACHE_DIR=~/.cache/alter/mise
MISE_STATE_DIR=~/.local/state/alter/mise/state
```

Plugin runtime config lives in `alter.mise.toml`, not `mise.toml`. If a
tool-versions style file is needed, it must be named `alter.tool-versions`. This
prevents user files such as `~/.tool-versions`, parent-directory `.tool-versions`,
or `~/.config/mise/config.toml` from influencing alter-managed plugin execution.
Both the singular and plural tool-versions override environment names are set because
current mise settings expose the plural form while the alter policy names the singular
form.
The environment passed to mise starts from a small allowlist (`HOME`, `PATH`, `TMPDIR`,
`TERM`, `LANG`, `LC_ALL`) and does not inherit mise/asdf activation variables.

Before `mise install`, alter reads only the plugin workspace `alter.mise.toml` and
`alter.tool-versions`. If neither declares tools, install is skipped and the adapter
runs in direct mode. The `hello` plugin currently declares no mise-managed tools, so
invoking `hello_greet` must not install unrelated global tools such as lua, node,
python, ruby, go, pnpm, or poetry.

The `test-runtime` plugin declares exactly one mise-managed tool, `node@24`, so its
runtime mode is `mise`. Running `alter test-runtime node-version` should install or
reuse only that declared Node.js runtime, then execute the adapter through `mise exec`
inside `plugins/test-runtime`.

Mise-managed plugins require explicit trust before execution. Trust is recorded in:

```text
~/.local/state/alter/trust/plugins.json
```

The trust store fingerprints:

- `alter.plugin.toml`
- `alter.mise.toml`, when present
- `alter.tool-versions`, when present
- adapter entrypoint file, when it exists inside the plugin workspace

Trust is invalidated when any trusted fingerprint changes, or when the plugin workspace
path changes. The next run refuses to execute and tells you to review and trust again.

Use:

```sh
alter plugin trust-status test-runtime
alter plugin trust test-runtime
alter plugin untrust test-runtime
```

`alter plugin trust <name>` shows a review summary and asks for confirmation with `huh`.
Trust is never written silently. The review means:

- `alter.mise.toml` is plugin-owned runtime policy. It declares tools mise may install
  or reuse for this plugin.
- Trusting it means accepting this local plugin directory, its runtime config, and its
  adapter entrypoint as code you are willing to run.
- Running untrusted code means mise may download tool archives and the adapter process
  may execute local commands with your user permissions.
- To trust it, inspect `alter.plugin.toml`, `alter.mise.toml`, and the adapter entrypoint;
  confirm declared tools and adapter code match your expectation; then run the command
  again. If it does not match, do not run that plugin.

Direct runtime plugins with no declared tools do not require trust. MCP mode cannot prompt
for trust, so it fails with a concise actionable error when a tool needs trust.

Set `ALTER_LOG=debug` to print runtime decision details to stderr. Debug output includes
plugin name, workspace, adapter entrypoint, runtime mode, runtime config presence,
declared tools, install skip status, mise path when used, mise cwd, sanitized mise
environment values, and exact commands. Debug logging does not print arbitrary inherited
environment variables.

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
test_runtime_node_version
```

Tool registration is intentionally explicit. The current path is:

```text
plugin metadata
-> tool registration
-> MCP exposure
-> adapter invocation
```

`hello_greet` calls the `hello` adapter's `greet` tool and `test_runtime_node_version`
calls the `test-runtime` adapter's `node-version` tool. Both return adapter JSON as text
content and structured content.

Future direction:

- derive MCP tools from adapter metadata
- expose more plugin tools after adapter metadata stabilizes
- keep MCP registration separate from plugin manifest parsing
- keep transport-specific code thin

Manual isolation check:

```sh
./bin/alter plugin doctor hello
./bin/alter plugin doctor test-runtime
./bin/alter plugin trust test-runtime
ALTER_LOG=debug ./bin/alter test-runtime node-version
npx -y @modelcontextprotocol/inspector ./bin/alter mcp
```

In the Inspector, invoking `hello_greet` should not install unrelated tools from
`~/.tool-versions`, `~/.config/mise/config.toml`, parent-directory mise files, or shell
activation state.

Expected `hello` doctor output includes:

```text
runtime mode: direct
mise install: skipped
declared tools: none
```

Expected `test-runtime` doctor output includes:

```text
runtime mode: mise
mise install: required
declared tools: node@24
user/global mise/asdf config: ignored
```

Invoking `test_runtime_node_version` in MCP Inspector should return a Node.js version
and must not install Python, Ruby, Go, Lua, pnpm, Poetry, or anything from user/global
mise or asdf config.

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
