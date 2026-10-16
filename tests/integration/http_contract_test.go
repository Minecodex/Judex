package integrationtest_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/kakj-go/Judex/internal/app"
	"github.com/kakj-go/Judex/internal/config"
	apigen "github.com/kakj-go/Judex/internal/gen/api"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestProductionHTTPResponsesMatchOpenAPI(t *testing.T) {
	openapi3filter.RegisterBodyDecoder("text/event-stream", openapi3filter.FileBodyDecoder)
	_, _, identities, pool := newDecisionEnv(t)
	objectsEndpoint := contractObjects(t)
	cfg, err := config.FromEnv(func(key string) string {
		switch key {
		case "JUDEX_DATABASE_URL":
			return pool.Config().ConnConfig.ConnString()
		case "JUDEX_ENV":
			return "test"
		case "JUDEX_MODE":
			return "api"
		case "JUDEX_S3_ENDPOINT":
			return objectsEndpoint
		case "JUDEX_S3_ACCESS_KEY", "JUDEX_S3_SECRET_KEY", "JUDEX_S3_BUCKET":
			return "contract"
		case "JUDEX_S3_PATH_STYLE":
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	application, err := app.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close(context.Background())
	doc, err := apigen.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range doc.Servers {
		if strings.HasPrefix(server.URL, "/") {
			server.URL = "http://judex.test" + server.URL
		}
	}
	routes, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	protectedCount := 0
	for template, item := range doc.Paths.Map() {
		for method, operation := range item.Operations() {
			if operation.Security == nil || len(*operation.Security) == 0 {
				continue
			}
			path := regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(template, uuid.NewString())
			request := httptest.NewRequest(method, "http://judex.test/api/v1"+path, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			route, params, err := routes.FindRoute(request)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			application.Server.Handler.ServeHTTP(response, request)
			if response.Code != 401 {
				t.Errorf("unauthenticated %s returned %d", operation.OperationID, response.Code)
				continue
			}
			check := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: request, PathParams: params, Route: route}, Status: response.Code, Header: response.Header(), Options: &openapi3filter.Options{IncludeResponseStatus: true}}
			check.SetBodyBytes(response.Body.Bytes())
			if err = openapi3filter.ValidateResponse(context.Background(), check); err != nil {
				t.Errorf("unauthenticated %s schema: %v", operation.OperationID, err)
			}
			protectedCount++
		}
	}
	t.Log("validated protected operations:", protectedCount)
	cookies := map[string]*http.Cookie{}
	csrf := ""
	operations := map[string]bool{}
	call := func(method, path string, body any) map[string]any {
		t.Helper()
		raw, _ := json.Marshal(body)
		binary, isBinary := body.([]byte)
		if isBinary {
			raw = binary
		}
		req := httptest.NewRequest(method, "http://judex.test/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		if isBinary {
			req.Header.Set("Content-Type", "application/octet-stream")
			sum := sha256.Sum256(raw)
			req.Header.Set("X-Judex-Part-SHA256", hex.EncodeToString(sum[:]))
		}
		req.Header.Set("Idempotency-Key", uuid.NewString())
		req.Header.Set("X-CSRF-Token", csrf)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		route, params, err := routes.FindRoute(req)
		if err != nil {
			t.Fatal(err)
		}
		requestValidation := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
		if err = openapi3filter.ValidateRequest(context.Background(), requestValidation); err != nil {
			t.Errorf("operation=%s request contract: %v", route.Operation.OperationID, err)
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		response := httptest.NewRecorder()
		if strings.HasSuffix(path, "/events") {
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			timer := time.AfterFunc(3*time.Second, cancel)
			defer timer.Stop()
			stopAfter := 2
			if strings.Contains(path, "/runs/") {
				stopAfter = 1
			}
			application.Server.Handler.ServeHTTP(&contractStreamRecorder{ResponseRecorder: response, cancel: cancel, stopAfter: stopAfter}, req.WithContext(ctx))
			if !strings.Contains(response.Body.String(), "data:") {
				t.Error("SSE returned no actual event frame")
			}
		} else {
			application.Server.Handler.ServeHTTP(response, req)
		}
		for _, cookie := range response.Result().Cookies() {
			cookies[cookie.Name] = cookie
		}
		if response.Code >= 400 {
			t.Fatalf("%s %s: %d %s", method, path, response.Code, response.Body.String())
		}
		validation := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}, Status: response.Code, Header: response.Header(), Options: &openapi3filter.Options{IncludeResponseStatus: true}}
		validation.SetBodyBytes(response.Body.Bytes())
		if err = openapi3filter.ValidateResponse(context.Background(), validation); err != nil {
			t.Errorf("operation=%s status=%d contract: %v", route.Operation.OperationID, response.Code, err)
		}
		operations[route.Operation.OperationID] = true
		if !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
			return map[string]any{"content": response.Body.String()}
		}
		if response.Code == 204 {
			return map[string]any{}
		}
		var envelope map[string]any
		if err = json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		data, _ := envelope["data"].(map[string]any)
		return data
	}
	call("GET", "/system", nil)
	call("GET", "/capabilities", nil)
	auth := call("POST", "/auth/register", map[string]any{"displayName": "Contract", "email": "contract@test.local", "password": "contract-password-123"})
	user := auth["user"].(map[string]any)
	session := call("GET", "/auth/session", nil)
	csrf = session["csrfToken"].(string)
	for _, path := range []string{"/projects", "/me/sessions", "/me/client-grants", "/me/invitations", "/me/actions", "/me/notifications", "/models"} {
		call("GET", path, nil)
	}
	p := call("POST", "/projects", map[string]any{"title": "Contract project"})
	prefix := "/projects/" + p["id"].(string)
	for _, path := range []string{"", "/bootstrap", "/members", "/positions", "/identities", "/workflows", "/topics", "/plans", "/tasks", "/proposals", "/handoffs", "/materials", "/uploads", "/repositories", "/release-reports", "/audit"} {
		call("GET", prefix+path, nil)
	}
	device := call("POST", "/auth/device/authorizations", map[string]any{"deviceName": "Contract CLI", "requestedScopes": []any{"projects:read"}, "projectScope": []any{p["id"]}})
	call("GET", "/auth/device/review?userCode="+device["userCode"].(string), nil)
	call("POST", "/auth/device/confirm", map[string]any{"userCode": device["userCode"], "approved": true, "scopes": []any{"projects:read"}, "projectScope": []any{p["id"]}})
	pair := call("POST", "/auth/device/token", map[string]any{"deviceCode": device["deviceCode"]})
	call("POST", "/auth/token/refresh", map[string]any{"refreshToken": pair["refreshToken"]})
	position := call("POST", prefix+"/positions", map[string]any{"name": "Reviewer", "prompt": "Review"})
	identity := call("POST", prefix+"/identities", map[string]any{"positionId": position["id"], "userId": user["id"]})
	prefs := call("GET", prefix+"/me/preferences", nil)
	call("PUT", prefix+"/me/preferences", map[string]any{"positionId": position["id"], "expectedRevision": prefs["items"].([]any)[0].(map[string]any)["revision"], "prompt": "private preference"})
	flow := call("POST", prefix+"/workflows", map[string]any{"name": "Contract workflow", "nodes": []any{map[string]any{"id": "review", "name": "Review", "responsibility": "Check proof", "allowedPositionIds": []any{position["id"]}, "defaultApprovalPolicy": "all"}}, "hardRules": []any{}, "advisoryEdges": []any{}, "approvalPolicies": map[string]any{}})
	flowPath := prefix + "/workflows/" + flow["id"].(string)
	versions := call("GET", flowPath+"/versions", nil)
	draftVersion := versions["items"].([]any)[0].(map[string]any)
	call("POST", flowPath+"/publish", map[string]any{"expectedVersion": flow["version"], "draftHash": draftVersion["draftHash"]})
	call("GET", flowPath+"/versions", nil)
	plan := call("POST", prefix+"/plans", map[string]any{"title": "Draft plan", "goal": "Goal", "acceptanceCriteria": "Criteria", "ownerIdentityId": identity["id"]})
	task := call("POST", prefix+"/tasks", map[string]any{"title": "Draft task", "reviewerIdentityId": identity["id"], "participantIdentityIds": []any{identity["id"]}})
	call("GET", prefix+"/plans/"+plan["id"].(string), nil)
	call("GET", prefix+"/tasks/"+task["id"].(string), nil)
	topic := call("POST", prefix+"/topics", map[string]any{"title": "Contract topic"})
	topicPath := prefix + "/topics/" + topic["id"].(string)
	call("GET", topicPath, nil)
	call("GET", topicPath+"/messages", nil)
	submission := call("POST", prefix+"/submissions", map[string]any{"clientSubmissionId": uuid.NewString(), "purpose": "message", "topicId": topic["id"], "text": "Contract message"})
	call("GET", topicPath+"/messages", nil)
	initialRun := call("POST", topicPath+"/runs", map[string]any{"sourceSubmissionId": submission["id"]})
	proposal := call("POST", prefix+"/proposals", map[string]any{"kind": "work_arrangement", "changes": []any{map[string]any{"operation": "create_task", "targetType": "task", "clientRef": "work", "fields": map[string]any{"title": "Approved task", "reviewerIdentityId": identity["id"], "participantIdentityIds": []any{identity["id"]}}}}})
	proposalPath := prefix + "/proposals/" + proposal["id"].(string)
	review := call("GET", proposalPath+"/review", nil)
	review = call("POST", proposalPath+"/submit", map[string]any{"expectedVersion": 1, "draftHash": review["reviewHash"]})
	slots := []any{}
	bindings := []any{}
	for _, raw := range review["slots"].([]any) {
		s := raw.(map[string]any)
		slots = append(slots, s["id"])
		bindings = append(bindings, map[string]any{"identityId": s["authorityId"], "bindingVersion": s["bindingVersion"]})
	}
	result := call("POST", proposalPath+"/decisions", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": 2, "decision": "approve", "slotIds": slots, "actingBindingVersions": bindings})
	taskID := result["createdIds"].(map[string]any)["work"].(string)
	taskPath := prefix + "/tasks/" + taskID
	started := call("POST", taskPath+"/start", map[string]any{"expectedVersion": 1, "identityId": identity["id"]})
	impact := call("GET", taskPath+"/execution-review?operation=skip", nil)
	call("POST", taskPath+"/skip", map[string]any{"expectedVersion": started["version"], "reviewHash": impact["reviewHash"], "reason": "Contract exception", "waivers": []any{}})
	impact = call("GET", taskPath+"/execution-review?operation=restore", nil)
	started = call("POST", taskPath+"/restore", map[string]any{"expectedVersion": impact["targetVersion"], "reviewHash": impact["reviewHash"], "reason": "Contract restoration", "waivers": []any{}})
	call("POST", taskPath+"/reports", map[string]any{"kind": "delivery", "text": "Proof", "identityId": identity["id"], "expectedTaskVersion": started["version"]})
	call("GET", taskPath+"/reports", nil)
	review = call("GET", taskPath+"/acceptance-review", nil)
	call("POST", taskPath+"/acceptances", map[string]any{"reviewId": review["reviewId"], "reviewHash": review["reviewHash"], "expectedVersion": review["targetVersion"], "decision": "accept"})
	draftPlan := call("POST", prefix+"/plans", map[string]any{"title": "Discard plan", "ownerIdentityId": identity["id"]})
	draftPlanPath := prefix + "/plans/" + draftPlan["id"].(string)
	call("PUT", draftPlanPath+"/draft", map[string]any{"expectedVersion": draftPlan["version"], "fields": map[string]any{"goal": "Updated goal"}})
	draftTask := call("POST", prefix+"/tasks", map[string]any{"title": "Discard task", "planId": draftPlan["id"]})
	draftTaskPath := prefix + "/tasks/" + draftTask["id"].(string)
	updatedDraft := call("PUT", draftTaskPath+"/draft", map[string]any{"expectedVersion": draftTask["version"], "fields": map[string]any{"title": "Revised task", "participantIdentityIds": []any{identity["id"]}, "reviewerIdentityId": identity["id"]}})
	discard := call("GET", draftTaskPath+"/discard-review", nil)
	call("POST", draftTaskPath+"/discard", map[string]any{"expectedVersion": updatedDraft["version"], "reviewHash": discard["reviewHash"]})
	discard = call("GET", draftPlanPath+"/discard-review", nil)
	call("POST", draftPlanPath+"/discard", map[string]any{"expectedVersion": discard["targetVersion"], "reviewHash": discard["reviewHash"]})
	for _, path := range []string{"/positions", "/identities", "/topics", "/plans", "/tasks", "/proposals", "/audit", "/execution-map"} {
		call("GET", prefix+path, nil)
	}
	type browserSession struct {
		cookies map[string]*http.Cookie
		csrf    string
	}
	savedSessions := map[string]browserSession{"contract@test.local": {cookies: cookies, csrf: csrf}}
	copyCookies := func(source map[string]*http.Cookie) map[string]*http.Cookie {
		result := map[string]*http.Cookie{}
		for key, value := range source {
			result[key] = value
		}
		return result
	}
	savedSessions["contract@test.local"] = browserSession{copyCookies(cookies), csrf}
	coverage := &contractScenario{t: t, call: call, prefix: prefix, user: user, position: position, identity: identity, flow: flow, taskPath: taskPath, topicPath: topicPath, refresh: func() { csrf = call("GET", "/auth/session", nil)["csrfToken"].(string) }, clearSessions: func() { clear(savedSessions) }, recovery: func() string {
		code, err := identities.IssueRecoveryCode(context.Background(), "contract@test.local", "contract-fixture", "Exercise public recovery schema")
		if err != nil {
			t.Fatal(err)
		}
		return code
	}, login: func(email string) {
		if saved, ok := savedSessions[email]; ok {
			cookies = copyCookies(saved.cookies)
			csrf = saved.csrf
			return
		}
		call("POST", "/auth/login", map[string]any{"email": email, "password": "contract-password-123"})
		csrf = call("GET", "/auth/session", nil)["csrfToken"].(string)
		savedSessions[email] = browserSession{copyCookies(cookies), csrf}
	}}
	coverage.pool = pool
	coverage.configuration()
	coverage.membership()
	coverage.workLifecycle()
	coverage.files(pool, objectsEndpoint)
	coverage.runtime(pool, initialRun)
	coverage.account(pair)
	missing := []string{}
	total := 0
	for _, item := range doc.Paths.Map() {
		for _, operation := range item.Operations() {
			total++
			if !operations[operation.OperationID] {
				missing = append(missing, operation.OperationID)
			}
		}
	}
	sort.Strings(missing)
	if file := os.Getenv("JUDEX_CONTRACT_EVIDENCE"); file != "" {
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed() && len(missing) == 0, "totalOperations": total, "successOperations": sortedOperations(operations), "anonymousProtectedOperations": protectedCount, "missingSuccessOperations": missing}, "", "  ")
		if err := os.WriteFile(file, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if len(missing) > 0 {
		t.Errorf("successful request/response contract gaps (%d/%d): %s", len(operations), total, strings.Join(missing, ","))
	}
	t.Log("validated operations:", len(operations), strings.Join(sortedOperations(operations), ","))
}
func sortedOperations(ops map[string]bool) []string {
	out := []string{}
	for op := range ops {
		out = append(out, op)
	}
	sort.Strings(out)
	return out
}
