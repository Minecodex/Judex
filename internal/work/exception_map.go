package work

import (
	"context"
	"github.com/google/uuid"
)

func (s *Service) decorateExecutionMap(ctx context.Context, user, project uuid.UUID, m *ExecutionMap) error {
	skipped := map[string]bool{}
	tasks := make([]Task, len(m.Nodes))
	for i, n := range m.Nodes {
		tasks[i].ID = parseUUID(n.TaskID)
	}
	if err := s.taskAccessPage(ctx, user, project, tasks); err != nil {
		return err
	}
	for i := range m.Nodes {
		e := tasks[i].ExecutionException
		m.Nodes[i].ExecutionException = e
		m.Nodes[i].Capabilities = tasks[i].Capabilities
		m.Nodes[i].DiscardedAt = tasks[i].DiscardedAt
		if e != nil {
			skipped[m.Nodes[i].TaskID] = true
		}
	}
	original := append([]MapEdge(nil), m.Edges...)
	incoming := map[string][]MapEdge{}
	for _, e := range original {
		if e.Kind != "parent" {
			incoming[e.ToTaskID] = append(incoming[e.ToTaskID], e)
		}
	}
	m.Edges = []MapEdge{}
	seen := map[string]bool{}
	var bridge func(MapEdge, []string, map[string]bool)
	bridge = func(e MapEdge, via []string, visited map[string]bool) {
		if !skipped[e.FromTaskID] {
			key := e.FromTaskID + e.ToTaskID + e.Phase + e.Kind
			if !seen[key] {
				e.ViaTaskIDs = via
				m.Edges = append(m.Edges, e)
				seen[key] = true
			}
			return
		}
		if visited[e.FromTaskID] {
			return
		}
		next := map[string]bool{}
		for id, v := range visited {
			next[id] = v
		}
		next[e.FromTaskID] = true
		for _, parent := range incoming[e.FromTaskID] {
			out := e
			out.FromTaskID = parent.FromTaskID
			if parent.Kind == "soft" {
				out.Kind = "soft"
			}
			bridge(out, append(append([]string{}, via...), e.FromTaskID), next)
		}
	}
	for _, e := range original {
		if e.Kind == "parent" {
			m.Edges = append(m.Edges, e)
			continue
		}
		if skipped[e.FromTaskID] || skipped[e.ToTaskID] {
			old := e
			old.Original = true
			m.Edges = append(m.Edges, old)
		}
		if !skipped[e.ToTaskID] {
			bridge(e, nil, map[string]bool{})
		}
	}
	return nil
}
