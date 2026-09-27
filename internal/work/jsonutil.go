// SPDX-License-Identifier: Apache-2.0

package work

import "encoding/json"

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func jsonUnmarshal(data []byte, target any) error { return json.Unmarshal(data, target) }
