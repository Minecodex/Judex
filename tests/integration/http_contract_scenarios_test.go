package integrationtest_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/infrastructure/postgres"
)

type contractScenario struct {
	pool                           *postgres.Pool
	t                              *testing.T
	call                           func(string, string, any) map[string]any
	login                          func(string)
	recovery                       func() string
	refresh, clearSessions         func()
	prefix                         string
	user, position, identity, flow map[string]any
	taskPath, topicPath            string
}

func (s *contractScenario) configuration() {
	c, p := s.call, s.prefix
	c("GET", "/me/recent-topics", nil)
	c("POST", "/me/notifications/read", map[string]any{"ids": []any{}})
	project := c("GET", p, nil)
	c("PATCH", p, map[string]any{"expectedVersion": project["version"], "description": "Contract updated"})
	repository := c("POST", p+"/repositories", map[string]any{"displayName": "Contract repository", "url": "https://example.test/repository", "provider": "git", "defaultBranch": "main"})
	c("PATCH", p+"/repositories/"+repository["id"].(string), map[string]any{"expectedVersion": repository["version"], "displayName": "Updated repository"})
	c("PATCH", p+"/positions/"+s.position["id"].(string), map[string]any{"expectedVersion": s.position["currentVersion"], "name": "Reviewer", "prompt": "Review", "publicSummary": "Contract public responsibility"})
	draft := map[string]any{"name": "Updated workflow", "nodes": []any{map[string]any{"id": "review", "name": "Review", "responsibility": "Check proof", "allowedPositionIds": []any{s.position["id"]}, "defaultApprovalPolicy": "all"}}, "hardRules": []any{}, "advisoryEdges": []any{}, "approvalPolicies": map[string]any{}}
	heads := c("GET", p+"/workflows", nil)
	head := heads["items"].([]any)[0].(map[string]any)
	c("GET", p+"/workflows/"+s.flow["id"].(string)+"/versions/"+head["publishedVersionId"].(string), nil)
	c("PUT", p+"/workflows/"+s.flow["id"].(string)+"/draft", map[string]any{"expectedVersion": head["version"], "body": draft})
	positions := c("GET", "/position-presets", nil)
	roleID := positions["scenarios"].([]any)[0].(map[string]any)["roleIds"].([]any)[0]
	c("POST", p+"/positions/import-presets", map[string]any{"catalogVersion": positions["version"], "scenarioId": "software", "roleIds": []any{roleID}, "locale": "en"})
	workflows := c("GET", "/workflow-presets", nil)
	c("POST", p+"/workflows/import-presets", map[string]any{"catalogVersion": workflows["version"], "scenarioId": "software", "workflowIds": []any{"software-standard"}, "locale": "en"})
	c("POST", s.taskPath+"/main-topic", map[string]any{})
	c("GET", s.taskPath+"/activity", nil)
	c("GET", p+"/deliveries", nil)
	c("GET", p+"/discussion-suggestions", nil)
	topic := c("GET", s.topicPath, nil)
	c("POST", s.topicPath+"/links", map[string]any{"expectedVersion": topic["linksVersion"], "targetRefs": []any{map[string]any{"objectType": "task", "objectId": s.taskPath[len(p+"/tasks/"):]}}})
	topic = c("GET", s.topicPath, nil)
	c("PUT", s.topicPath+"/links", map[string]any{"expectedLinksVersion": topic["linksVersion"], "targetRefs": []any{}})
	c("POST", s.topicPath+"/fork", map[string]any{"title": "Contract fork", "forkAfterSeq": 1, "links": []any{}})
	c("POST", p+"/release-reports", map[string]any{"versionLabel": "contract.1", "environment": "isolated", "status": "success", "repositoryCommits": []any{}})
	body := map[string]any{"title": "Global intent contract"}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	intent := c("POST", "/confirmation-intents", map[string]any{"operation": "project.create", "objectId": uuid.NewString(), "reviewHash": hex.EncodeToString(sum[:]), "payload": body})
	intent = c("GET", "/confirmation-intents/"+intent["id"].(string), nil)
	c("POST", "/confirmation-intents/"+intent["id"].(string)+"/confirm", map[string]any{"intentHash": intent["intentHash"], "decision": "approve"})
	forkPayload := map[string]any{"title": "Review fork", "forkAfterSeq": 1}
	raw, _ = json.Marshal(forkPayload)
	sum = sha256.Sum256(raw)
	intent = c("POST", p+"/confirmation-intents", map[string]any{"operation": "topic.fork", "objectId": s.topicPath[len(p+"/topics/"):], "reviewHash": hex.EncodeToString(sum[:]), "payload": forkPayload})
	intent = c("GET", p+"/confirmation-intents/"+intent["id"].(string), nil)
	c("POST", p+"/confirmation-intents/"+intent["id"].(string)+"/confirm", map[string]any{"intentHash": intent["intentHash"], "decision": "reject"})
	project = c("GET", p, nil)
	project = c("POST", p+"/archive", map[string]any{"expectedVersion": project["version"], "reason": "Contract archival"})
	c("POST", p+"/restore", map[string]any{"expectedVersion": project["version"], "reason": "Contract restore"})
}

func (s *contractScenario) membership() {
	c, p := s.call, s.prefix
	member := c("POST", "/auth/register", map[string]any{"displayName": "Contract guest", "email": "guest@contract.test", "password": "contract-password-123"})["user"].(map[string]any)
	s.login("contract@test.local")
	invitation := c("POST", p+"/invitations", map[string]any{"targetEmail": "guest@contract.test", "positionIds": []any{}})
	c("GET", p+"/invitations", nil)
	c("GET", "/invitations/resolve?token="+strings.TrimPrefix(invitation["inviteUrl"].(string), "/invite/"), nil)
	s.login("guest@contract.test")
	offers := c("GET", "/me/invitations", nil)
	_ = offers
	c("POST", "/invitations/"+invitation["id"].(string)+"/accept", map[string]any{"expectedVersion": 1})
	s.login("contract@test.local")
	c("PATCH", p+"/members/"+member["id"].(string), map[string]any{"expectedVersion": c("GET", p, nil)["version"], "role": "manager"})
	c("POST", p+"/identities/"+s.identity["id"].(string)+"/replace", map[string]any{"expectedBindingVersion": s.identity["currentBindingVersion"], "newUserId": s.user["id"], "reason": "Explicit same-holder replacement contract"})
	project := c("GET", p, nil)
	transfer := c("POST", p+"/owner-transfer", map[string]any{"targetUserId": member["id"], "expectedVersion": project["version"]})
	c("GET", p+"/owner-transfers/"+transfer["id"].(string), nil)
	s.login("guest@contract.test")
	c("POST", p+"/owner-transfers/"+transfer["id"].(string)+"/decisions", map[string]any{"expectedVersion": transfer["projectVersion"], "decision": "decline"})
	c("POST", p+"/leave", map[string]any{"expectedVersion": c("GET", p, nil)["version"]})
	s.login("contract@test.local")
	invitation = c("POST", p+"/invitations", map[string]any{"targetEmail": "guest@contract.test", "positionIds": []any{}})
	s.login("guest@contract.test")
	c("POST", "/invitations/"+invitation["id"].(string)+"/decline", map[string]any{"expectedVersion": 1})
	s.login("contract@test.local")
	invitation = c("POST", p+"/invitations", map[string]any{"targetEmail": "revoke@contract.test", "positionIds": []any{}})
	c("POST", p+"/invitations/"+invitation["id"].(string)+"/revoke", map[string]any{"expectedVersion": 1})
	invitation = c("POST", p+"/invitations", map[string]any{"targetEmail": "guest@contract.test", "positionIds": []any{}})
	s.login("guest@contract.test")
	c("POST", "/invitations/"+invitation["id"].(string)+"/accept", map[string]any{"expectedVersion": 1})
	s.login("contract@test.local")
	c("POST", p+"/members/"+member["id"].(string)+"/remove", map[string]any{"expectedVersion": c("GET", p, nil)["version"], "reason": "Remove fixture guest"})
}
func (s *contractScenario) account(pair map[string]any) {
	c := s.call
	s.login("contract@test.local")
	c("POST", "/auth/login", map[string]any{"email": "contract@test.local", "password": "contract-password-123"})
	s.refresh()
	user := c("GET", "/auth/session", nil)["user"].(map[string]any)
	c("PATCH", "/me", map[string]any{"expectedVersion": user["version"], "displayName": "Contract updated", "locale": "en"})
	sessions := c("GET", "/me/sessions", nil)["items"].([]any)
	for _, raw := range sessions {
		row := raw.(map[string]any)
		if row["isCurrent"] != true && row["current"] != true {
			c("DELETE", "/me/sessions/"+row["id"].(string), nil)
			break
		}
	}
	grants := c("GET", "/me/client-grants", nil)["items"].([]any)
	if len(grants) > 0 {
		c("DELETE", "/me/client-grants/"+grants[0].(map[string]any)["id"].(string), nil)
	}
	c("POST", "/me/password", map[string]any{"currentPassword": "contract-password-123", "newPassword": "contract-password-456"})
	s.refresh()
	// Return to the same password so the public login helper remains reusable.
	c("POST", "/auth/logout", map[string]any{})
	c("POST", "/auth/login", map[string]any{"email": "contract@test.local", "password": "contract-password-456"})
	s.refresh()
	// A fresh session is necessary after password rotation; the next login helper
	// refreshes CSRF after the recovery endpoint has invalidated all old sessions.
	c("POST", "/auth/recover", map[string]any{"recoveryCode": s.recovery(), "newPassword": "contract-password-123"})
	s.clearSessions()
	s.login("contract@test.local")
	c("POST", "/auth/logout-all", map[string]any{"expectedAuthVersion": 3})
}
