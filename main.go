package main

import (
	"github.com/alexferrari88/code2context/cmd"
	"github.com/alexferrari88/code2context/internal/utils"
)

func main() {
	cmd.Execute()
}

// Initialize global logger
func init() {
	// Default to non-verbose. Cobra PersistentPreRun will set it based on flag.
	utils.InitLogger(false)
}
