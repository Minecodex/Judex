package cli

import (
	"fmt"
	"github.com/kakj-go/Judex/internal/work"
	"github.com/spf13/cobra"
	"strings"
)

func taskExecutionCommand(operation string) *cobra.Command {
	var reason string
	var selected []string
	var acknowledged bool
	cmd := &cobra.Command{Use: operation + " TASK", Args: cobra.ExactArgs(1), Short: "预览运行例外并通过浏览器确认一次", RunE: func(cmd *cobra.Command, args []string) error {
		c, err := NewClient()
		if err != nil {
			return err
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		prefix := "/projects/" + project
		var review work.ExceptionReview
		if err = c.Do(cmd.Context(), "GET", prefix+"/tasks/"+args[0]+"/execution-review?operation="+operation, nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		if strings.TrimSpace(reason) == "" {
			return emit(review, nil)
		}
		waivers := []work.ExceptionWaiver{}
		for _, key := range selected {
			found := false
			for _, task := range review.AffectedTasks {
				for _, r := range task.Requirements {
					if key == task.TaskID.String()+":"+r.ID.String() && r.Hard {
						waivers = append(waivers, work.ExceptionWaiver{TaskID: task.TaskID, RequirementID: r.ID, Fingerprint: r.Fingerprint})
						found = true
					}
				}
			}
			if !found {
				return fmt.Errorf("unknown or non-waivable requirement %s", key)
			}
		}
		payload := work.ExceptionCommand{ExpectedVersion: review.TargetVersion, ReviewHash: review.ReviewHash, Reason: reason, Waivers: waivers, AcknowledgeStarted: acknowledged}
		var intent map[string]any
		if err = c.Do(cmd.Context(), "POST", prefix+"/confirmation-intents", map[string]any{"operation": "task." + operation, "objectId": args[0], "reviewHash": review.ReviewHash, "payload": payload}, &intent, requestKey()); err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, prefix+"/confirmation-intents", intent)
	}}
	cmd.Flags().StringVar(&reason, "reason", "", "原因；为空时只显示影响清单")
	cmd.Flags().StringArrayVar(&selected, "waive", nil, "明确豁免的后继任务ID:条件ID，可重复")
	cmd.Flags().BoolVar(&acknowledged, "acknowledge-started", false, "恢复时明确保留已经开展的后继并重新核对")
	return cmd
}
