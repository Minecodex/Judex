// SPDX-License-Identifier: Apache-2.0

package project

import "encoding/json"

func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }
