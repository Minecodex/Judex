package cli

import (
	"errors"
	"net/url"
	"os"
	"strconv"

	"github.com/kakj-go/Judex/pkg/client"
	"github.com/spf13/cobra"
)

var groupFlag string

func Root() *cobra.Command {
	root := &cobra.Command{Use: "judex", Short: "Judex V2: group collaboration and local work", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&requestIDFlag, "request-id", "", "Stable idempotency UUID")
	root.PersistentFlags().StringVar(&projectFlag, "project", "", "Project ID")
	root.PersistentFlags().StringVar(&groupFlag, "group", "", "Group ID")
	root.PersistentFlags().StringVar(&serverFlag, "server", "", "Server URL")
	root.PersistentFlags().StringVar(&profileFlag, "profile", "default", "Credential profile")
	root.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Machine-readable V2 envelope")
	authCmd := &cobra.Command{Use: "auth"}
	authCmd.AddCommand(authLoginCommand(), authWhoamiCommand(), authLogoutCommand())
	projects := &cobra.Command{Use: "project"}
	projects.AddCommand(readCommand("list", func() (string, error) { return "/projects", nil }), readCommand("show", func() (string, error) { id, e := currentProject(); return "/projects/" + id, e }), selectCommand(false))
	groups := &cobra.Command{Use: "group"}
	groups.AddCommand(readCommand("list", func() (string, error) { id, e := currentProject(); return "/projects/" + id + "/groups", e }), readCommand("show", func() (string, error) { id, e := currentGroup(); return "/groups/" + id, e }), selectCommand(true))
	messages := &cobra.Command{Use: "message"}
	messages.AddCommand(messagesCommand(), sendCommand(false))
	files := &cobra.Command{Use: "file"}
	files.AddCommand(readCommand("list", func() (string, error) { id, e := currentGroup(); return "/conversations/" + id + "/files", e }), uploadCommand(), downloadCommand())
	contextCmd := &cobra.Command{Use: "context"}
	contextCmd.AddCommand(contextCommand())
	runs := &cobra.Command{Use: "run"}
	runs.AddCommand(readCommand("list", func() (string, error) { id, e := currentGroup(); return "/groups/" + id + "/runs", e }))
	workflow := &cobra.Command{Use: "workflow"}
	workflow.AddCommand(readCommand("list", func() (string, error) { id, e := currentProject(); return "/projects/" + id + "/workflows", e }))
	skill := &cobra.Command{Use: "skill"}
	skill.AddCommand(skillInstallCommand(), skillUninstallCommand())
	root.AddCommand(skill)

	root.AddCommand(versionCommand(), statusCommand(), authCmd, projects, groups, messages, files, contextCmd, runs, workflow, sendCommand(true), watchCommand(), readCommand("inbox", func() (string, error) { return "/me/notifications", nil }), pendingCommand())
	return root
}
func currentProject() (string, error) {
	if projectFlag != "" {
		return projectFlag, nil
	}
	id := LoadProfile().ProjectID
	if id == "" {
		return "", errors.New("select a project: judex project use ID")
	}
	return id, nil
}
func currentGroup() (string, error) {
	if groupFlag != "" {
		return groupFlag, nil
	}
	id := LoadProfile().GroupID
	if id == "" {
		return "", errors.New("select a group: judex group use ID")
	}
	return id, nil
}
func requestKey() string {
	if requestIDFlag != "" {
		return requestIDFlag
	}
	return client.NewKey()
}
func readCommand(name string, path func() (string, error)) *cobra.Command {
	return &cobra.Command{Use: name, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		p, e := path()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		var out any
		e = c.Do(cmd.Context(), "GET", p, nil, &out, "")
		return emit(out, e)
	}}
}
func selectCommand(group bool) *cobra.Command {
	return &cobra.Command{Use: "use ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		profile := LoadProfile()
		path := "/projects/" + args[0]
		if group {
			path = "/groups/" + args[0]
		}
		var out struct{ ID, ProjectID string }
		e = c.Do(cmd.Context(), "GET", path, nil, &out, "")
		if e != nil {
			return emit(nil, e)
		}
		if group {
			profile.GroupID = args[0]
			profile.ProjectID = out.ProjectID
		} else {
			profile.ProjectID = args[0]
			profile.GroupID = ""
		}
		profile.Server = c.Server
		e = SaveProfile(profile)
		return emit(profile, e)
	}}
}
func messagesCommand() *cobra.Command {
	var after, before int64
	var search string
	cmd := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		var out any
		e = c.Do(cmd.Context(), "GET", "/conversations/"+id+"/messages?after="+strconv.FormatInt(after, 10)+"&before="+strconv.FormatInt(before, 10)+"&q="+url.QueryEscape(search), nil, &out, "")
		return emit(out, e)
	}}
	cmd.Flags().Int64Var(&after, "after", 0, "After message sequence")
	cmd.Flags().Int64Var(&before, "before", 0, "Before message sequence")
	cmd.Flags().StringVar(&search, "search", "", "Search permitted message text")
	return cmd
}
func sendCommand(push bool) *cobra.Command {
	var text, replyTo, confirmation, decision string
	var files, ais, people []string
	var record bool
	name := "send"
	if push {
		name = "push"
	}
	cmd := &cobra.Command{Use: name, RunE: func(cmd *cobra.Command, args []string) error {
		id, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		if text == "" && len(files) == 0 {
			return emit(nil, errors.New("--text or --file required"))
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		mentions := []map[string]any{}
		for _, rid := range ais {
			mentions = append(mentions, map[string]any{"kind": "role_agent", "targetId": rid})
		}
		for _, uid := range people {
			mentions = append(mentions, map[string]any{"kind": "human", "targetId": uid, "requiresResponse": confirmation != ""})
		}
		path := "/conversations/" + id + "/messages"
		if push {
			path = "/groups/" + id + "/push"
		}
		var out any
		body := map[string]any{"body": text, "fileVersionIds": []string{}, "mentions": mentions, "recordOnly": record}
		if replyTo != "" {
			body["replyTo"] = replyTo
		}
		if confirmation != "" {
			body["confirmation"] = map[string]any{"title": confirmation}
		}
		if decision != "" {
			body["decision"] = map[string]any{"choice": decision, "expectedVersion": 1}
		}
		if len(files) > 0 {
			out, e = sendWithFiles(cmd.Context(), c, id, path, body, files, requestIDFlag)
		} else {
			e = c.Do(cmd.Context(), "POST", path, body, &out, requestKey())
		}
		return emit(out, e)
	}}
	cmd.Flags().StringVar(&text, "text", "", "Message text")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply to a message ID in this conversation; a reply alone does not wake AI")
	cmd.Flags().StringSliceVar(&files, "file", nil, "Local files to upload")
	if push {
		cmd.Flags().BoolVar(&record, "record-only", false, "Save without waking any role")
	} else {
		cmd.Flags().StringSliceVar(&ais, "mention-ai", nil, "Exact role-agent IDs to wake")
		cmd.Flags().StringSliceVar(&people, "mention-human", nil, "Exact human user IDs to notify")
		cmd.Flags().StringVar(&confirmation, "confirmation", "", "Request a human decision with this title and --mention-human recipients")
		cmd.Flags().StringVar(&decision, "decision", "", "Record agree, changes, or comment for --reply-to; AI wakes only with --mention-ai")
	}
	return cmd
}
func contextCommand() *cobra.Command {
	var after int64
	var kind, ref string
	var offset, limit int
	cmd := &cobra.Command{Use: "read", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		path := "/groups/" + id + "/context?after=" + strconv.FormatInt(after, 10) + "&limit=" + strconv.Itoa(limit)
		if ref != "" {
			path = "/groups/" + id + "/context/reference?kind=" + url.QueryEscape(kind) + "&ref=" + url.QueryEscape(ref) + "&offset=" + strconv.Itoa(offset) + "&limit=" + strconv.Itoa(limit)
		}
		var out any
		e = c.Do(cmd.Context(), "GET", path, nil, &out, "")
		return emit(out, e)
	}}
	cmd.Flags().Int64Var(&after, "after", 0, "After knowledge sequence")
	cmd.Flags().StringVar(&kind, "kind", "", "Reference kind")
	cmd.Flags().StringVar(&ref, "ref", "", "Exact public reference")
	cmd.Flags().IntVar(&offset, "offset", 0, "Reference text offset")
	cmd.Flags().IntVar(&limit, "limit", 50, "Page size")
	return cmd
}
func watchCommand() *cobra.Command {
	var after int64
	var limit int
	cmd := &cobra.Command{Use: "watch", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := currentGroup()
		if e != nil {
			return emit(nil, e)
		}
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		return c.FollowEvents(cmd.Context(), id, after, limit, func(event client.StreamEvent) error { return emit(event, nil) })
	}}
	cmd.Flags().Int64Var(&after, "after", 0, "Resume after event sequence")
	cmd.Flags().IntVar(&limit, "limit", 0, "Stop after N events; zero follows continuously")
	return cmd
}
func pendingCommand() *cobra.Command {
	return &cobra.Command{Use: "pending", RunE: func(cmd *cobra.Command, args []string) error {
		c, e := NewClient()
		if e != nil {
			return emit(nil, e)
		}
		out, e := c.PendingCommands()
		if e == nil {
			var operations []map[string]any
			operations, e = pendingOperations(c)
			out = append(out, operations...)
		}
		return emit(out, e)
	}}
}

var _ = os.Stdin
