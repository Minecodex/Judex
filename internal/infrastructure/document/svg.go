package document

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// SVG is displayed only as an image. Reject active and external content before
// publishing it, and never inject an uploaded SVG into the application DOM.
func ValidateSVG(raw []byte) error {
	if len(raw) > 2<<20 {
		return fmt.Errorf("SVG preview size limit")
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	root := false
	count := 0
	allowed := " svg g defs path rect circle ellipse line polyline polygon text tspan textPath use symbol title desc linearGradient radialGradient stop clipPath mask pattern marker "
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch v := token.(type) {
		case xml.Directive:
			return fmt.Errorf("SVG directives unsupported")
		case xml.ProcInst:
			if v.Target != "xml" {
				return fmt.Errorf("SVG processing instruction unsupported")
			}
		case xml.StartElement:
			count++
			if count > 20000 {
				return fmt.Errorf("SVG element limit")
			}
			if !root {
				if v.Name.Local != "svg" {
					return fmt.Errorf("SVG root required")
				}
				root = true
			}
			if !strings.Contains(allowed, " "+v.Name.Local+" ") {
				return fmt.Errorf("unsupported SVG element")
			}
			for _, a := range v.Attr {
				name, value := strings.ToLower(a.Name.Local), strings.ToLower(strings.TrimSpace(a.Value))
				if strings.HasPrefix(name, "on") || name == "src" || name == "href" && !strings.HasPrefix(value, "#") || strings.Contains(value, "url(") && !strings.HasPrefix(value, "url(#") || strings.Contains(value, "javascript:") || strings.Contains(value, "@import") {
					return fmt.Errorf("active SVG content unsupported")
				}
			}
		}
	}
	if !root {
		return fmt.Errorf("empty SVG")
	}
	return nil
}
