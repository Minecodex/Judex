package catalog

import (
	_ "embed"
	"encoding/json"
)

//go:embed workflow-presets.json
var workflowJSON []byte

type WorkflowNode struct {
	Kind           string `json:"kind,omitempty"`
	Phase          *Text  `json:"phase,omitempty"`
	ID             string `json:"id"`
	Name           Text   `json:"name"`
	Responsibility Text   `json:"responsibility"`
}
type WorkflowEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind,omitempty"`
	Label *Text  `json:"label,omitempty"`
}
type WorkflowSource struct {
	Title Text   `json:"title"`
	URL   string `json:"url"`
}
type WorkflowPreset struct {
	Complexity   string           `json:"complexity,omitempty"`
	Sources      []WorkflowSource `json:"sources,omitempty"`
	ID           string           `json:"id"`
	Name         Text             `json:"name"`
	Summary      Text             `json:"summary"`
	Instructions Text             `json:"instructions"`
	Nodes        []WorkflowNode   `json:"nodes"`
	Edges        []WorkflowEdge   `json:"edges"`
	RoleIDs      []string         `json:"roleIds"`
	ScenarioIDs  []string         `json:"scenarioIds"`
}
type WorkflowScenario struct {
	ID          string   `json:"id"`
	Name        Text     `json:"name"`
	Description Text     `json:"description"`
	WorkflowIDs []string `json:"workflowIds"`
}
type WorkflowCatalog struct {
	Version   string             `json:"version"`
	Workflows []WorkflowPreset   `json:"workflows"`
	Scenarios []WorkflowScenario `json:"scenarios"`
	Roles     []WorkflowRole     `json:"roles"`
}

type WorkflowRole struct {
	ID   string `json:"id"`
	Name Text   `json:"name"`
}

// Scenario names and descriptions are always read from the position catalog.
func Workflows() WorkflowCatalog {
	var value WorkflowCatalog
	if err := json.Unmarshal(workflowJSON, &value); err != nil {
		panic("invalid embedded workflow catalog: " + err.Error())
	}
	positions := Positions()
	for _, role := range positions.Roles {
		value.Roles = append(value.Roles, WorkflowRole{ID: role.ID, Name: role.Name})
	}
	for _, scenario := range positions.Scenarios {
		item := WorkflowScenario{ID: scenario.ID, Name: scenario.Name, Description: scenario.Description, WorkflowIDs: []string{}}
		for _, preset := range value.Workflows {
			for _, id := range preset.ScenarioIDs {
				if id == scenario.ID {
					item.WorkflowIDs = append(item.WorkflowIDs, preset.ID)
				}
			}
		}
		value.Scenarios = append(value.Scenarios, item)
	}
	return value
}
