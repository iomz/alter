package main

import (
	"context"
	"reflect"
	"sort"
	"testing"
)

func TestRootCommandSurface(t *testing.T) {
	cmd := newCommand()
	var got []string
	for _, child := range cmd.Commands() {
		got = append(got, child.Name())
	}
	sort.Strings(got)
	want := []string{"hello", "mcp", "plugin", "setup", "test-runtime"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("root commands = %v, want %v", got, want)
	}
	if !cmd.CompletionOptions.DisableDefaultCmd {
		t.Fatal("default completion command enabled")
	}
}

func TestPluginCommandSurface(t *testing.T) {
	cmd := newPluginCommand()
	var got []string
	for _, child := range cmd.Commands() {
		got = append(got, child.Name())
	}
	sort.Strings(got)
	want := []string{"doctor", "inspect", "list", "trust", "trust-status", "untrust"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plugin commands = %v, want %v", got, want)
	}
}

func TestPluginCommandsRequireOnePluginName(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		usage string
	}{
		{name: "inspect", args: []string{"plugin", "inspect"}, usage: "usage: alter plugin inspect <name>"},
		{name: "doctor", args: []string{"plugin", "doctor"}, usage: "usage: alter plugin doctor <name>"},
		{name: "trust", args: []string{"plugin", "trust"}, usage: "usage: alter plugin trust <name>"},
		{name: "untrust", args: []string{"plugin", "untrust"}, usage: "usage: alter plugin untrust <name>"},
		{name: "trust-status", args: []string{"plugin", "trust-status"}, usage: "usage: alter plugin trust-status <name>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCommand()
			cmd.SetArgs(tt.args)
			err := cmd.ExecuteContext(context.Background())
			if err == nil || err.Error() != tt.usage {
				t.Fatalf("ExecuteContext() error = %v, want %q", err, tt.usage)
			}
		})
	}
}

func TestHelloNameDefault(t *testing.T) {
	cmd := newHelloGreetCommand()
	got, err := cmd.Flags().GetString("name")
	if err != nil {
		t.Fatal(err)
	}
	if got != "world" {
		t.Fatalf("name default = %q, want world", got)
	}
}
