package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "commitlog-ai",
	Short: "See the prompts behind every git commit",
	Long:  `commitlog-ai connects your AI agent conversations (Claude Code, Gemini CLI, Codex CLI) to your git history.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
