// Package catalog owns the shared scenario and preset content used by project
// positions, workflows and the explicit browser demo.
package catalog

import (
	_ "embed"
	"encoding/json"
)

//go:embed position-presets.json
var positionJSON []byte

type Text struct {
	Zh string `json:"zh"`
	En string `json:"en"`
}

func (text Text) Localized(locale string) string {
	if locale == "en" {
		return text.En
	}
	return text.Zh
}

type PositionPreset struct {
	ID      string `json:"id"`
	Name    Text   `json:"name"`
	Summary Text   `json:"summary"`
	Prompt  Text   `json:"prompt"`
}
type PositionScenario struct {
	ID          string   `json:"id"`
	Name        Text     `json:"name"`
	Description Text     `json:"description"`
	RoleIDs     []string `json:"roleIds"`
}
type PositionCatalog struct {
	Version   string             `json:"version"`
	Roles     []PositionPreset   `json:"roles"`
	Scenarios []PositionScenario `json:"scenarios"`
}

func Positions() PositionCatalog {
	var value PositionCatalog
	if err := json.Unmarshal(positionJSON, &value); err != nil {
		panic("invalid embedded position catalog: " + err.Error())
	}
	return value
}
