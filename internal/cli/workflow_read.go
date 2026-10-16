package cli

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func workflowReadCommand() *cobra.Command {
	var version string
	cmd := &cobra.Command{Use: "get WORKFLOW_ID", Short: "读取固定已发布流程版本及正文哈希", Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		if id, err := uuid.Parse(args[0]); err != nil || id == uuid.Nil {
			return fmt.Errorf("invalid workflow UUID")
		}
		if id, err := uuid.Parse(version); err != nil || id == uuid.Nil {
			return fmt.Errorf("invalid workflow version UUID")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		definition := uuid.MustParse(args[0])
		fixed := uuid.MustParse(version)
		project, err := currentProject()
		if err != nil {
			return emit(nil, err)
		}
		c, err := NewClient()
		if err != nil {
			return emit(nil, err)
		}
		var data map[string]any
		err = c.Do(cmd.Context(), "GET", "/projects/"+project+"/workflows/"+definition.String()+"/versions/"+fixed.String(), nil, &data, "")
		return emit(data, err)
	}}
	cmd.Flags().StringVar(&version, "version", "", "已发布版本UUID")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}
