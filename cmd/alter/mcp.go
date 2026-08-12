package main

import (
	"io"
	"os"

	"github.com/iomz/alter/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "serve MCP over stdio",
		RunE: func(cmd *cobra.Command, _ []string) error {
			executor, err := executorContextWithRuntimeOutput(io.Discard, os.Stderr)
			if err != nil {
				return err
			}
			return mcp.Serve(cmd.Context(), executor)
		},
	}
}
