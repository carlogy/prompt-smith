package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// copyToClipboard puts text on the system clipboard. A package variable
// so tests can substitute a spy instead of touching the real clipboard.
var copyToClipboard = clipboard.WriteAll

// generateOptions holds the root command's flag values.
type generateOptions struct {
	target       string
	skills       []string
	context      string
	constraints  string
	role         string
	outputFormat string
	copy         bool
	out          string
}

// addGenerateFlags registers the generate flags on cmd and wires its RunE.
func addGenerateFlags(cmd *cobra.Command, reg *registry.Registry) {
	opts := &generateOptions{}

	cmd.Flags().StringVarP(&opts.target, "target", "t", "generic", "target harness: generic|opencode|claude-code")
	cmd.Flags().StringSliceVarP(&opts.skills, "skills", "s", nil, "skills to include (comma-separated or repeatable)")
	cmd.Flags().StringVar(&opts.context, "context", "", "background/context for the goal")
	cmd.Flags().StringVar(&opts.constraints, "constraints", "", "constraints the solution must respect")
	cmd.Flags().StringVar(&opts.role, "role", "", "role/persona to open the prompt with")
	cmd.Flags().StringVar(&opts.outputFormat, "output-format", "", "desired shape of the response")
	cmd.Flags().BoolVarP(&opts.copy, "copy", "c", false, "copy the prompt to the clipboard instead of stdout")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "write the prompt to this file instead of stdout")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd, reg, opts, args)
	}
}

func runGenerate(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, args []string) error {
	goal := strings.TrimSpace(strings.Join(args, " "))
	if goal == "" {
		return fmt.Errorf(`promptsmith: a goal is required, e.g. promptsmith "fix the flaky test"`)
	}

	if len(opts.skills) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: no --skills given; generating a goal-only prompt (interactive skill picker arrives in a later release)")
	}

	out, err := prompt.Build(reg, prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
	})
	if err != nil {
		return err
	}

	return deliver(cmd, opts, out)
}

// deliver routes the assembled prompt to every requested destination
// (file, clipboard), additively; if none were requested, it prints to
// stdout.
func deliver(cmd *cobra.Command, opts *generateOptions, out string) error {
	delivered := false

	if opts.out != "" {
		if err := os.WriteFile(opts.out, []byte(out+"\n"), 0o644); err != nil {
			return fmt.Errorf("promptsmith: write %s: %w", opts.out, err)
		}
		delivered = true
	}

	if opts.copy {
		if err := copyToClipboard(out); err != nil {
			return fmt.Errorf("promptsmith: copy to clipboard: %w", err)
		}
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: copied to clipboard")
		delivered = true
	}

	if !delivered {
		cmd.Println(out)
	}
	return nil
}
