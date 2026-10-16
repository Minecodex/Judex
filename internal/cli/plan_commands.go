package cli

import "github.com/spf13/cobra"

// Read the existing public plan and frozen acceptance-review contracts before
// preparing an owner's decision. Neither command creates an intent or work.
func planReadCommand(review bool) *cobra.Command {
	name, short := "show", "读取正式计划、负责人、状态和版本"
	if review {
		name, short = "review", "读取计划固定验收快照、任务接受事实和阻塞条件"
	}
	return &cobra.Command{Use: name + " ID", Args: cobra.ExactArgs(1), Short: short, RunE: func(cmd *cobra.Command, args []string) error {
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		route := "/projects/" + project + "/plans/" + args[0]
		if review {
			route += "/acceptance-review"
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "GET", route, nil, &result, "")
		return emit(result, err)
	}}
}
