// SPDX-License-Identifier: Apache-2.0
package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// reportedError preserves partially successful command data already emitted.
// Unwrap keeps the public API error classification available to the exit mapper.
type reportedError struct{ cause error }

func (e *reportedError) Error() string { return e.cause.Error() }
func (e *reportedError) Unwrap() error { return e.cause }

// Run is the single process boundary for Cobra and business-command errors.
// Root creates fresh flag defaults on every invocation; no exit status survives
// into a later invocation. Commands still emit their existing result envelope.
func Run(args []string) int {
	root := Root()
	makeCommandGroupsRunnable(root)
	started := false
	markCommandStart(root, &started)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	var reported *reportedError
	if !errors.As(err, &reported) {
		// Cobra may stop parsing before a later --json. Respect the requested
		// output mode even then, while ignoring flag values and text after --.
		jsonFlag = requestedJSON(root, args)
		_ = emit(nil, err)
	}
	var apiErr *client.CLIError
	if errors.As(err, &apiErr) {
		if code := apiErr.ExitCode(); code != 0 {
			return code
		}
	}
	var syntax *json.SyntaxError
	var valueType *json.UnmarshalTypeError
	if !started || errors.As(err, &syntax) || errors.As(err, &valueType) || errors.Is(err, os.ErrNotExist) {
		return 2
	}
	return 1
}

// Cobra otherwise treats a non-runnable group followed by an unknown argument
// as help with exit 0. Give groups an explicit help action and argument check
// so invalid nested commands pass through the same structured error boundary.
func makeCommandGroupsRunnable(cmd *cobra.Command) {
	if cmd.HasSubCommands() && cmd.Run == nil && cmd.RunE == nil {
		if cmd.Args == nil {
			cmd.Args = cobra.NoArgs
		}
		cmd.RunE = func(command *cobra.Command, args []string) error { return command.Help() }
	}
	for _, child := range cmd.Commands() {
		makeCommandGroupsRunnable(child)
	}
}

func markCommandStart(cmd *cobra.Command, started *bool) {
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(command *cobra.Command, args []string) error {
			*started = true
			return run(command, args)
		}
	}
	if run := cmd.Run; run != nil {
		cmd.Run = func(command *cobra.Command, args []string) {
			*started = true
			run(command, args)
		}
	}
	for _, child := range cmd.Commands() {
		markCommandStart(child, started)
	}
}

func requestedJSON(root *cobra.Command, args []string) bool {
	flags, short := map[string]*pflag.Flag{}, map[string]*pflag.Flag{}
	var visit func(*cobra.Command)
	visit = func(cmd *cobra.Command) {
		for _, set := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags()} {
			set.VisitAll(func(flag *pflag.Flag) {
				flags[flag.Name] = flag
				if flag.Shorthand != "" {
					short[flag.Shorthand] = flag
				}
			})
		}
		for _, child := range cmd.Commands() {
			visit(child)
		}
	}
	visit(root)
	mode := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if name == "json" && strings.HasPrefix(arg, "--") {
			mode = true
			if inline {
				if explicit, err := strconv.ParseBool(value); err == nil {
					mode = explicit
				}
			}
			continue
		}
		flag := flags[name]
		if !strings.HasPrefix(arg, "--") {
			flag = short[strings.TrimPrefix(arg, "-")]
		}
		if flag != nil && flag.NoOptDefVal == "" && !inline {
			i++ // A string flag's literal value is not another option.
		}
	}
	return mode
}
