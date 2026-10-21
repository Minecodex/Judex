package document

import "testing"

func TestSVGPreviewBoundary(t *testing.T) {
	for _, value := range []string{`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path d="M0 0h20v20z"/></svg>`, `<svg><defs><linearGradient id="g"><stop offset="0" stop-color="red"/></linearGradient></defs><rect fill="url(#g)" width="20" height="20"/></svg>`} {
		if e := ValidateSVG([]byte(value)); e != nil {
			t.Fatal(e)
		}
	}
	for _, value := range []string{`<svg><script>alert(1)</script></svg>`, `<svg onload="alert(1)"/>`, `<svg><use href="https://example.org/secret"/></svg>`, `<!DOCTYPE svg><svg/>`, `<svg><foreignObject/></svg>`, `<html/>`, `<svg><rect fill="url(https://example.org)"/></svg>`} {
		if e := ValidateSVG([]byte(value)); e == nil {
			t.Fatalf("accepted %s", value)
		}
	}
}
