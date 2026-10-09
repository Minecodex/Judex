package cli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"strconv"
	"time"
)

func listCommand(description string, route func() (string, error)) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{Use: "list", Short: description, RunE: func(cmd *cobra.Command, args []string) error {
		if limit < 1 || limit > 100 {
			return emit(nil, fmt.Errorf("limit must be 1..100"))
		}
		path, err := route()
		if err != nil {
			return emit(nil, err)
		}
		parsed, err := url.Parse(path)
		if err != nil {
			return emit(nil, err)
		}
		query := parsed.Query()
		query.Set("limit", strconv.Itoa(limit))
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		parsed.RawQuery = query.Encode()
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var page map[string]any
		if err = c.Do(cmd.Context(), "GET", parsed.String(), nil, &page, ""); err != nil {
			return emit(nil, err)
		}
		return emit(page, nil)
	}}
	cmd.Flags().IntVar(&limit, "limit", 50, "每页条数 1–100")
	cmd.Flags().StringVar(&cursor, "cursor", "", "上一页 nextCursor")
	return cmd
}
func eventsWatchCommand() *cobra.Command {
	var after int64
	var limit int
	var timeout time.Duration
	cmd := &cobra.Command{Use: "watch", Short: "读取可恢复的项目事件流", RunE: func(cmd *cobra.Command, args []string) error {
		if after < 0 || limit < 1 || limit > 500 || timeout <= 0 {
			return emit(nil, fmt.Errorf("invalid event cursor/limit/timeout"))
		}
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		page, err := c.Events(ctx, project, after, limit)
		return emit(page, err)
	}}
	cmd.Flags().Int64Var(&after, "after", 0, "最后处理的事件序号")
	cmd.Flags().IntVar(&limit, "limit", 50, "最多返回的事件数")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Second, "最长等待时间")
	return cmd
}
