package tools

import "context"

type runtimeKey struct{}
type RuntimeAuthority struct{ Project, Run, Lease, ToolCall string }

func WithRuntimeAuthority(ctx context.Context, authority RuntimeAuthority) context.Context {
	return context.WithValue(ctx, runtimeKey{}, authority)
}
func Authority(ctx context.Context) (RuntimeAuthority, bool) {
	a, ok := ctx.Value(runtimeKey{}).(RuntimeAuthority)
	return a, ok
}
