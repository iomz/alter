package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServerExposesDefaultTools(t *testing.T) {
	ctx := context.Background()
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	server := NewServer(&fakeInvoker{out: []byte(`{"message":"hello, iomz","plugin":"hello"}`)})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Wait()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("tools = %#v, want two tools", tools.Tools)
	}
	got := map[string]bool{}
	for _, tool := range tools.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"hello_greet", "test_runtime_node_version"} {
		if !got[want] {
			t.Fatalf("tools = %#v, missing %s", tools.Tools, want)
		}
	}
}

func TestHelloGreetCallsAdapter(t *testing.T) {
	ctx := context.Background()
	invoker := &fakeInvoker{out: []byte(`{"message":"hello, iomz","plugin":"hello"}`)}
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	server := NewServer(invoker)

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Wait()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "hello_greet",
		Arguments: map[string]any{"name": "iomz"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if invoker.plugin != "hello" || invoker.tool != "greet" {
		t.Fatalf("called %s/%s, want hello/greet", invoker.plugin, invoker.tool)
	}
	if invoker.args["name"] != "iomz" {
		t.Fatalf("args = %#v, want name iomz", invoker.args)
	}
	if len(result.Content) != 1 {
		t.Fatalf("content length = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want TextContent", result.Content[0])
	}
	if text.Text != string(invoker.out) {
		t.Fatalf("content text = %q, want adapter JSON", text.Text)
	}

	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		data, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &structured); err != nil {
			t.Fatal(err)
		}
	}
	if structured["message"] != "hello, iomz" {
		t.Fatalf("structuredContent = %#v, want message", structured)
	}
}

func TestRuntimeNodeVersionCallsAdapter(t *testing.T) {
	ctx := context.Background()
	invoker := &fakeInvoker{out: []byte(`{"plugin":"test-runtime","nodeVersion":"v24.0.0"}`)}
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	server := NewServer(invoker)

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Wait()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "test_runtime_node_version",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if invoker.plugin != "test-runtime" || invoker.tool != "node-version" {
		t.Fatalf("called %s/%s, want test-runtime/node-version", invoker.plugin, invoker.tool)
	}
	text, ok := result.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want TextContent", result.Content[0])
	}
	if text.Text != string(invoker.out) {
		t.Fatalf("content text = %q, want adapter JSON", text.Text)
	}
}

type fakeInvoker struct {
	out    []byte
	plugin string
	tool   string
	args   map[string]any
}

func (i *fakeInvoker) Invoke(plugin, tool string, args any) ([]byte, error) {
	i.plugin = plugin
	i.tool = tool
	i.args, _ = args.(map[string]any)
	return i.out, nil
}
