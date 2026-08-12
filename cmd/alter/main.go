package main

import (
	"context"
	"fmt"
	"os"

	"github.com/iomz/alter/internal/ui"
)

func main() {
	if err := newCommand().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error("error"), err)
		os.Exit(1)
	}
}
