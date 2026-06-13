# AGENTS.md

## Purpose

`alter` is a local/private tool control plane for managed CLI plugins and MCP exposure.

The core responsibility of alter is:

- plugin discovery
- plugin execution
- runtime resolution
- MCP exposure
- registry/index management

Business logic belongs in plugins or upstream tools.

---

## Design Philosophy

alter is a control plane.

It is not:

- an agent framework
- a workflow engine
- a business application
- a data processing platform

Keep the scope focused.

When in doubt, prefer moving functionality into plugins rather than expanding the core.

---

## Responsibility Boundaries

External tools should not be required to implement alter-specific interfaces.

Adapters exist to translate external tools into the alter contract.

Preferred structure:

alter
→ adapter
→ upstream tool

Never push alter-specific requirements into upstream projects.

Do not modify upstream projects solely to improve alter integration.

---

## Core vs Plugin

The alter core may contain:

- plugin discovery
- plugin lifecycle management
- runtime resolution
- MCP exposure
- registry/index handling
- execution orchestration

The alter core must not contain:

- ingest logic
- domain-specific integrations
- business workflows
- service-specific behavior

Those belong in plugins.

---

## Runtime Policy

mise is the only required external dependency.

alter may:

- bootstrap mise interactively
- execute plugins through `mise exec`
- manage plugin-local runtime environments

alter must not:

- silently install software
- modify shell startup files automatically
- require shell activation
- depend on user-specific shell configuration

Shell integration, if ever provided, should be optional and explicit.

---

## Plugin Philosophy

Plugins should be self-contained.

A plugin adapter is responsible for:

- locating the upstream tool
- validating prerequisites
- exposing commands to alter
- translating results into the alter contract

Upstream tools are not required to know that alter exists.

Maintain a clear separation between:

- alter
- plugin adapters
- upstream tools

Avoid coupling across those layers.