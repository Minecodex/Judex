package cli

import (
	"fmt"
	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"strings"
)

func decisionCommand(approve bool) *cobra.Command {
	var hash, reason, reasonFile string
	name := "reject"
	if approve {
		name = "approve"
	}
	cmd := &cobra.Command{Use: name + " ID", Args: cobra.ExactArgs(1), Short: "创建浏览器确认意图，读取同一结果", RunE: func(cmd *cobra.Command, args []string) error {
		if reasonFile != "" {
			raw, err := os.ReadFile(reasonFile)
			if err != nil {
				return emit(nil, err)
			}
			reason = strings.TrimSpace(string(raw))
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var review map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/proposals/"+args[0]+"/review", nil, &review, ""); err != nil {
			return emit(nil, err)
		}
		if review["reviewHash"] != hash {
			return emit(nil, fmt.Errorf("review changed; read it again"))
		}
		if !approve && reason == "" {
			return emit(nil, fmt.Errorf("rejection reason required"))
		}
		slots := []any{}
		bindings := []any{}
		if rows, ok := review["slots"].([]any); ok {
			for _, raw := range rows {
				slot, ok := raw.(map[string]any)
				if !ok || slot["canDecide"] != true {
					continue
				}
				slots = append(slots, slot["id"])
				if slot["authorityType"] == "identity" {
					bindings = append(bindings, map[string]any{"identityId": slot["authorityId"], "bindingVersion": slot["bindingVersion"]})
				}
			}
		}
		if len(slots) == 0 {
			return emit(nil, fmt.Errorf("no pending responsibility held by this account"))
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "POST", "/projects/"+project+"/confirmation-intents", map[string]any{"operation": "proposal.decision", "objectId": args[0], "reviewHash": hash, "payload": map[string]any{"decision": name, "reason": reason, "reviewId": review["reviewId"], "slotIds": slots, "actingBindingVersions": bindings}}, &result, requestKey())
		if err != nil {
			return emit(nil, err)
		}
		return emitIntent(cmd, c, "/projects/"+project+"/confirmation-intents", result)
	}}
	cmd.Flags().StringVar(&hash, "review", "", "fixed review hash")
	_ = cmd.MarkFlagRequired("review")
	cmd.Flags().StringVar(&reason, "reason", "", "decision reason")
	cmd.Flags().StringVar(&reasonFile, "reason-file", "", "包含拒绝理由的 UTF-8 文件")
	return cmd
}
func intentResultCommand() *cobra.Command {
	var global bool
	cmd := &cobra.Command{Use: "result ID", Args: cobra.ExactArgs(1), Short: "读取同一浏览器决定结果", RunE: func(cmd *cobra.Command, args []string) error {
		prefix := "/confirmation-intents"
		if !global {
			project, err := currentProject()
			if err != nil {
				return emit(nil, err)
			}
			prefix = "/projects/" + project + prefix
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var result map[string]any
		err = c.Do(cmd.Context(), "GET", prefix+"/"+args[0], nil, &result, "")
		return emit(result, err)
	}}
	cmd.Flags().BoolVar(&global, "global", false, "查询建项目的用户级意图")
	return cmd
}

func contextCommand() *cobra.Command {
	var task, topic string
	cmd := &cobra.Command{Use: "get [TASK]", Args: cobra.MaximumNArgs(1), Short: "读取正式任务或议题上下文", RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			task = args[0]
		}
		if (task == "") == (topic == "") {
			return emit(nil, fmt.Errorf("choose one --task or --topic"))
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		path := "/projects/" + project + "/tasks/" + task
		if topic != "" {
			path = "/projects/" + project + "/topics/" + topic
		}
		var data map[string]any
		err = c.Do(cmd.Context(), "GET", path, nil, &data, "")
		if err != nil {
			return emit(nil, err)
		}
		if task != "" {
			data["discussionPath"] = "/projects/" + project + "/tasks/" + task + "/chat"
			if main, ok := data["mainTopicId"].(string); ok && main != "" {
				data["discussionPath"] = data["discussionPath"].(string) + "/" + main
			}
			var activities map[string]any
			if err = c.Do(cmd.Context(), "GET", path+"/activity?limit=20", nil, &activities, ""); err != nil {
				return emit(nil, err)
			}
			data["activities"] = activities
			var suggestions map[string]any
			if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/discussion-suggestions?taskId="+task, nil, &suggestions, ""); err != nil {
				return emit(nil, err)
			}
			data["discussionSuggestions"] = suggestions
			if plan, ok := data["planId"].(string); ok && plan != "" {
				var planValue map[string]any
				if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/plans/"+plan, nil, &planValue, ""); err != nil {
					return emit(nil, err)
				}
				data["mainTopicId"] = planValue["mainTopicId"]
			}
		} else {
			var history map[string]any
			if err = c.Do(cmd.Context(), "GET", path+"/messages?limit=50", nil, &history, ""); err != nil {
				return emit(nil, err)
			}
			data["discussionPath"] = "/projects/" + project + "/chat/" + topic
			data["history"] = history
		}
		return emit(data, nil)
	}}
	cmd.Flags().StringVar(&task, "task", "", "任务 ID")
	cmd.Flags().StringVar(&topic, "topic", "", "议题 ID")
	return cmd
}

type sourceSnapshot struct {
	ID               string  `json:"id"`
	TaskID           string  `json:"sourceTaskId"`
	CurrentVersionID *string `json:"currentVersionId"`
	CurrentVersion   *int64  `json:"currentVersion"`
}

func loadSource(cmd *cobra.Command, c *client.Client, project, source string) (sourceSnapshot, error) {
	cursor := ""
	for {
		var page struct {
			Items []struct {
				Sources []sourceSnapshot `json:"sources"`
			} `json:"items"`
			NextCursor string `json:"nextCursor"`
		}
		path := "/projects/" + project + "/handoffs?limit=100"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		if err := c.Do(cmd.Context(), "GET", path, nil, &page, ""); err != nil {
			return sourceSnapshot{}, err
		}
		for _, handoff := range page.Items {
			for _, item := range handoff.Sources {
				if item.ID == source {
					return item, nil
				}
			}
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			break
		}
		cursor = page.NextCursor
	}
	return sourceSnapshot{}, fmt.Errorf("handoff source not found")
}
