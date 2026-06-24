package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Invoker interface {
	Invoke(context.Context, string, string, any) ([]byte, error)
}

type ToolMetadata struct {
	Name        string
	Description string
	Plugin      string
	Tool        string
}

type HelloGreetArgs struct {
	Name string `json:"name" jsonschema:"Name to greet"`
}

type TestRuntimeNodeVersionArgs struct{}

func NewServer(invoker Invoker) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "alter",
		Version: "0.1.0",
	}, nil)
	RegisterTools(server, invoker, DefaultTools())
	return server
}

func Serve(ctx context.Context, invoker Invoker) error {
	return NewServer(invoker).Run(ctx, &mcpsdk.StdioTransport{})
}

func RegisterTools(server *mcpsdk.Server, invoker Invoker, tools []ToolMetadata) {
	for _, tool := range tools {
		registerTool(server, invoker, tool)
	}
}

func DefaultTools() []ToolMetadata {
	return []ToolMetadata{
		{
			Name:        "hello_greet",
			Description: "Return greeting JSON from hello adapter",
			Plugin:      "hello",
			Tool:        "greet",
		},
		{
			Name:        "test_runtime_node_version",
			Description: "Return Node.js version from test-runtime adapter",
			Plugin:      "test-runtime",
			Tool:        "node-version",
		},
	}
}

func registerTool(server *mcpsdk.Server, invoker Invoker, tool ToolMetadata) {
	switch tool.Name {
	case "hello_greet":
		mcpsdk.AddTool(server, &mcpsdk.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, args HelloGreetArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := invoker.Invoke(ctx, tool.Plugin, tool.Tool, map[string]any{"name": args.Name})
			if err != nil {
				return nil, nil, err
			}
			result, err := structuredOutput(out)
			if err != nil {
				return nil, nil, err
			}
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: string(out)},
				},
				StructuredContent: result,
			}, nil, nil
		})
	case "test_runtime_node_version":
		mcpsdk.AddTool(server, &mcpsdk.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ TestRuntimeNodeVersionArgs) (*mcpsdk.CallToolResult, any, error) {
			out, err := invoker.Invoke(ctx, tool.Plugin, tool.Tool, map[string]any{})
			if err != nil {
				return nil, nil, err
			}
			result, err := structuredOutput(out)
			if err != nil {
				return nil, nil, err
			}
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: string(out)},
				},
				StructuredContent: result,
			}, nil, nil
		})
	default:
		panic(fmt.Sprintf("unsupported MCP tool metadata %q", tool.Name))
	}
}

func structuredOutput(out []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode adapter JSON for MCP structured output: %w", err)
	}
	return result, nil
}
