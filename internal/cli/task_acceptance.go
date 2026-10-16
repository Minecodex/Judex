package cli

import (
	"fmt"
	"strings"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
	"github.com/spf13/cobra"
)

func taskAcceptCommand() *cobra.Command { return taskAcceptanceCommand(true) }
func taskRejectCommand() *cobra.Command { return taskAcceptanceCommand(false) }

func acceptanceHistoryCommand(kind string) *cobra.Command {
	var id string
	command := listCommand("读取验收历史、退回理由和原审阅来源", func() (string, error) {
		project, err := currentProject()
		return fmt.Sprintf("/projects/%s/%s/%s/acceptances", project, kind, id), err
	})
	command.Use = "acceptances ID"
	command.Args = cobra.ExactArgs(1)
	command.PreRun = func(_ *cobra.Command, args []string) { id = args[0] }
	return command
}

func taskAcceptanceCommand(accept bool) *cobra.Command {
	decision := "reject"
	if accept {
		decision = "accept"
	}
	var reason string
	command := &cobra.Command{Use: decision + " ID", Short: "取得同一成果快照并请求浏览器确认，拒收须给出原因", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !accept && strings.TrimSpace(reason) == "" {
			return emit(nil, apierrors.Fields("reason", "required"))
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		client, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var review map[string]any
		if err = client.Do(cmd.Context(), "GET", fmt.Sprintf("/projects/%s/tasks/%s/acceptance-review", project, args[0]), nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		payload := map[string]any{"decision": decision, "expectedVersion": review["targetVersion"], "reviewId": review["reviewId"]}
		if !accept {
			payload["reason"] = strings.TrimSpace(reason)
		}
		intent := map[string]any{"operation": "task.acceptance", "objectId": args[0], "reviewHash": review["reviewHash"], "payload": payload}
		prefix := "/projects/" + project + "/confirmation-intents"
		var created map[string]any
		if err = client.Do(cmd.Context(), "POST", prefix, intent, &created, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, client, prefix, created)
	}}
	if !accept {
		command.Flags().StringVar(&reason, "reason", "", "具体退回理由，最终仍由本人浏览器确认")
	}
	return command
}
