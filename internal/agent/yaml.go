// SPDX-License-Identifier: Apache-2.0

package agent

import "gopkg.in/yaml.v3"

func parseYAML(raw []byte, target any) error { return yaml.Unmarshal(raw, target) }
