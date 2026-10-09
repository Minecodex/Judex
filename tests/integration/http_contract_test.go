package integrationtest_test

import (
	"bytes"
	"context"
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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestProductionHTTPResponsesMatchOpenAPI(t *testing.T) {
	_, _, _, pool := newDecisionEnv(t)
	cfg, err := config.FromEnv(func(key string) string {
		switch key {
		case "JUDEX_DATABASE_URL":
			return pool.Config().ConnConfig.ConnString()
		case "JUDEX_ENV":
			return "test"
		case "JUDEX_MODE":
			return "api"
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
		req := httptest.NewRequest(method, "http://judex.test/api/v1"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
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
		application.Server.Handler.ServeHTTP(response, req)
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
	call("POST", topicPath+"/runs", map[string]any{"sourceSubmissionId": submission["id"]})
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
	if len(operations) < 35 {
		t.Fatal("insufficient contract coverage: " + strconv.Itoa(len(operations)))
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
