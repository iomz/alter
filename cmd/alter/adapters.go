package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newHelloCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hello",
		Short: "run hello adapter",
	}
	cmd.AddCommand(newHelloGreetCommand())
	return cmd
}

func newHelloGreetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "greet",
		Short: "return greeting JSON",
		RunE: func(cmd *cobra.Command, _ []string) error {
			executor, err := executorContext()
			if err != nil {
				return err
			}
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			out, err := executor.Invoke(cmd.Context(), "hello", "greet", map[string]any{"name": name})
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, string(out))
			return nil
		},
	}
	cmd.Flags().String("name", "world", "name to greet")
	return cmd
}

func newTestRuntimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test-runtime",
		Short: "run mise runtime isolation test adapter",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "node-version",
		Short: "return Node.js version resolved through mise mode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			executor, err := executorContext()
			if err != nil {
				return err
			}
			out, err := executor.Invoke(cmd.Context(), "test-runtime", "node-version", map[string]any{})
			if err != nil {
				return err
			}
			fmt.Fprint(os.Stdout, string(out))
			return nil
		},
	})
	return cmd
}
