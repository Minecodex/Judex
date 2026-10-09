package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func collaborationIntent(cmd *cobra.Command, operation, object string, payload map[string]any) error {
	project, err := currentProject()
	if err != nil {
		return emit(nil, err)
	}
	c, err := NewClient()
	if err != nil {
		return emit(nil, err)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return emit(nil, err)
	}
	sum := sha256.Sum256(raw)
	var out map[string]any
	prefix := "/projects/" + project + "/confirmation-intents"
	err = c.Do(cmd.Context(), "POST", prefix, map[string]any{"operation": operation, "objectId": object, "reviewHash": hex.EncodeToString(sum[:]), "payload": payload}, &out, requestKey())
	if err != nil {
		return emit(nil, err)
	}
	return emitIntent(cmd, c, prefix, out)
}
func topicForkCommand() *cobra.Command {
	var file, title string
	var point int64
	cmd := &cobra.Command{Use: "fork TOPIC", Args: cobra.ExactArgs(1), Short: "从固定消息位置分叉，浏览器确认一次", RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{}
		if file != "" {
			raw, err := os.ReadFile(file)
			if err != nil {
				return emit(nil, err)
			}
			if err = json.Unmarshal(raw, &payload); err != nil {
				return emit(nil, err)
			}
		}
		if title != "" {
			payload["title"] = title
		}
		if cmd.Flags().Changed("after-seq") {
			payload["forkAfterSeq"] = point
		}
		if value, _ := payload["title"].(string); value == "" {
			return emit(nil, fmt.Errorf("title is required"))
		}
		if _, ok := payload["forkAfterSeq"]; !ok {
			project, err := currentProject()
			if err != nil {
				return emit(nil, err)
			}
			c, err := NewClient()
			if err != nil {
				return emit(nil, err)
			}
			var topic map[string]any
			if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/topics/"+args[0], nil, &topic, ""); err != nil {
				return emit(nil, err)
			}
			payload["forkAfterSeq"] = topic["lastMessageSeq"]
		}
		return collaborationIntent(cmd, "topic.fork", args[0], payload)
	}}
	cmd.Flags().StringVar(&file, "file", "", "fork JSON（title/forkAfterSeq/links）")
	cmd.Flags().StringVar(&title, "title", "", "新分支问题")
	cmd.Flags().Int64Var(&point, "after-seq", 0, "固定分叉消息序号")
	return cmd
}
func discussionSuggestionCommands() *cobra.Command {
	group := &cobra.Command{Use: "discussion-suggestion", Short: "任务分析产生的讨论建议"}
	list := listCommand("分页查询讨论建议", func() (string, error) {
		project, err := currentProject()
		if err != nil {
			return "", err
		}
		return "/projects/" + project + "/discussion-suggestions", nil
	})
	var file, mode, title, topic string
	resolve := &cobra.Command{Use: "resolve ID", Args: cobra.ExactArgs(1), Short: "选择单独讨论、主讨论或已有讨论，浏览器确认一次", RunE: func(cmd *cobra.Command, args []string) error {
		payload := map[string]any{}
		if file != "" {
			raw, err := os.ReadFile(file)
			if err != nil {
				return emit(nil, err)
			}
			if err = json.Unmarshal(raw, &payload); err != nil {
				return emit(nil, err)
			}
		}
		if mode != "" {
			payload["mode"] = mode
		}
		if title != "" {
			payload["title"] = title
		}
		if topic != "" {
			payload["topicId"] = topic
		}
		if value, _ := payload["mode"].(string); value == "" {
			return emit(nil, fmt.Errorf("mode is required: create/main/link/dismiss"))
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var suggestion map[string]any
		if err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/discussion-suggestions/"+args[0], nil, &suggestion, ""); err != nil {
			return emit(nil, err)
		}
		if _, ok := payload["expectedVersion"]; !ok {
			payload["expectedVersion"] = suggestion["version"]
		}
		return collaborationIntent(cmd, "discussion_suggestion.resolve", args[0], payload)
	}}
	resolve.Flags().StringVar(&file, "file", "", "resolution JSON")
	resolve.Flags().StringVar(&mode, "mode", "", "create/main/link/dismiss")
	resolve.Flags().StringVar(&title, "title", "", "单独讨论的标题")
	resolve.Flags().StringVar(&topic, "topic", "", "link 目标讨论 ID")
	group.AddCommand(list, resolve)
	return group
}

func taskActivityCommand() *cobra.Command {
	var task string
	cmd := listCommand("分页读取完整任务记录", func() (string, error) {
		project, err := currentProject()
		return "/projects/" + project + "/tasks/" + task + "/activity", err
	})
	cmd.Use = "activity TASK"
	cmd.Args = cobra.ExactArgs(1)
	cmd.PreRun = func(cmd *cobra.Command, args []string) { task = args[0] }
	return cmd
}
func taskAnalysisCommands() *cobra.Command {
	group := &cobra.Command{Use: "task-analysis", Short: "任务上报的公开分析"}
	for _, op := range []string{"show", "retry"} {
		name := op
		cmd := &cobra.Command{Use: name + " ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			project, err := currentProject()
			if err != nil {
				return emit(nil, err)
			}
			c, err := NewClient()
			if err != nil {
				return emit(nil, err)
			}
			path := "/projects/" + project + "/task-analyses/" + args[0]
			method := "GET"
			var body any
			var key string
			if name == "retry" {
				method = "POST"
				path += "/retry"
				body = map[string]any{}
				key = requestKey()
			}
			var out map[string]any
			err = c.Do(cmd.Context(), method, path, body, &out, key)
			return emit(out, err)
		}}
		group.AddCommand(cmd)
	}
	return group
}
