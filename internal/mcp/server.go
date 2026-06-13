package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/iomz/alter/internal/plugin"
	"github.com/iomz/alter/internal/runtime"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func Serve(in io.Reader, out io.Writer, store *plugin.Store, runner *runtime.MiseRunner) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readFrame(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req request
		if err := json.Unmarshal(body, &req); err != nil {
			return err
		}
		if req.ID == nil {
			continue
		}
		res := handle(req, store, runner)
		if err := writeFrame(out, res); err != nil {
			return err
		}
	}
}

func handle(req request, store *plugin.Store, runner *runtime.MiseRunner) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "alter", "version": "0.1.0"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		res.Result = map[string]any{
			"tools": []map[string]any{{
				"name":        "hello_greet",
				"description": "Return greeting JSON from hello adapter",
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"name": map[string]any{"type": "string"}},
					"required":   []string{"name"},
				},
			}},
		}
	case "tools/call":
		result, err := callTool(req.Params, store, runner)
		if err != nil {
			res.Error = map[string]any{"code": -32000, "message": err.Error()}
			return res
		}
		res.Result = result
	default:
		res.Error = map[string]any{"code": -32601, "message": "method not found"}
	}
	return res
}

func callTool(raw json.RawMessage, store *plugin.Store, runner *runtime.MiseRunner) (any, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	if params.Name != "hello_greet" {
		return nil, fmt.Errorf("unknown tool %q", params.Name)
	}
	name, _ := params.Arguments["name"].(string)
	payload, err := json.Marshal(map[string]any{
		"tool": "greet",
		"args": map[string]any{"name": name},
	})
	if err != nil {
		return nil, err
	}
	p, err := store.Load("hello")
	if err != nil {
		return nil, err
	}
	out, err := runner.Run(p, "invoke", string(payload))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(out)}},
	}, nil
}

func readFrame(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q", line)
		}
		if strings.EqualFold(key, "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeFrame(out io.Writer, v any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(v); err != nil {
		return err
	}
	payload := bytes.TrimSpace(body.Bytes())
	_, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(payload), payload)
	return err
}
