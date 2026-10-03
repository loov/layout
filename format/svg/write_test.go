package svg

import (
	"strings"
	"testing"
)

func TestSanitizeHTMLRemovesObfuscatedScriptURLs(t *testing.T) {
	for _, url := range []string{
		"javascript:alert(1)", "java&#x09;script:alert(1)",
		"java&#x0a;script:alert(1)", "java&#x0d;script:alert(1)",
		"&#x01;JaVaScRiPt:alert(1)", "da&#x09;ta:text/html,unsafe",
	} {
		t.Run(url, func(t *testing.T) {
			got := sanitizeHTML(`<a href="` + url + `">click</a>`)
			if strings.Contains(got, "href=") || !strings.Contains(got, "click") {
				t.Fatalf("unsafe URL retained: %s", got)
			}
		})
	}
}

func TestSanitizeHTMLPreservesSafeLinks(t *testing.T) {
	for _, url := range []string{"https://example.com/", "/relative", "#anchor", "mailto:someone@example.com"} {
		t.Run(url, func(t *testing.T) {
			got := sanitizeHTML(`<a href="` + url + `">click</a>`)
			if !strings.Contains(got, `href="`+url+`"`) {
				t.Fatalf("safe link removed: %s", got)
			}
		})
	}
}

func TestSanitizeHTMLRemovesRawTextElements(t *testing.T) {
	for _, tag := range []string{"noscript", "xmp", "noembed", "noframes", "plaintext"} {
		t.Run(tag, func(t *testing.T) {
			got := sanitizeHTML(`<b>ok</b><` + tag + `><img src="x" onerror="alert(1)"/></` + tag + `>`)
			if strings.Contains(got, "onerror") || strings.Contains(got, "<img") {
				t.Fatalf("raw text survived: %s", got)
			}
			if !strings.Contains(got, "<b>ok</b>") {
				t.Fatalf("safe markup removed: %s", got)
			}
		})
	}
}
