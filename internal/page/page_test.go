package page

import (
	"strconv"
	"strings"
	"testing"
)

func TestPageSelfContained(t *testing.T) {
	s := string(HTML())

	// 1. HTML() is non-empty, starts with <!DOCTYPE html>, ContentLength()
	//    equals the length of HTML() as a decimal string.
	if s == "" {
		t.Fatal("HTML() is empty")
	}
	if !strings.HasPrefix(s, "<!DOCTYPE html>") {
		t.Fatalf("HTML() does not start with <!DOCTYPE html>: %q", s[:40])
	}
	if got := ContentLength(); got != strconv.Itoa(len(s)) {
		t.Fatalf("ContentLength() = %q, want %q", got, strconv.Itoa(len(s)))
	}

	// 2. The page makes no external request: no http://, no https://, no
	//    <link tag, no @import.
	if strings.Contains(s, "http://") {
		t.Error("page contains http:// — external request")
	}
	if strings.Contains(s, "https://") {
		t.Error("page contains https:// — external request")
	}
	if strings.Contains(s, "<link") {
		t.Error("page contains <link — external asset reference")
	}
	if strings.Contains(s, "@import") {
		t.Error("page contains @import — external style fetch")
	}

	// 3. The restart control is present: id="token", /restart, "Bearer ".
	if !strings.Contains(s, `id="token"`) {
		t.Error("page missing id=\"token\" — restart token input absent")
	}
	if !strings.Contains(s, "/restart") {
		t.Error("page missing /restart — restart endpoint reference absent")
	}
	if !strings.Contains(s, "Bearer ") {
		t.Error("page missing \"Bearer \" — restart auth token absent")
	}

	// 4. The restart POST is a POST: method: "POST".
	if !strings.Contains(s, `method: "POST"`) {
		t.Error("page missing method: \"POST\" — restart is not a POST")
	}

	// 5. The drawer's colSpan matches the table width — colSpan = 7 and
	//    exactly 7 <th in the <thead> row.
	if !strings.Contains(s, "colSpan = 7") {
		t.Error("page missing colSpan = 7 — drawer not wide enough")
	}
	// Count <th> and <th … (excludes <thead>).
	thCount := strings.Count(s, "<th>") + strings.Count(s, "<th ")
	if thCount != 7 {
		t.Fatalf("page has %d <th occurrences, want 7", thCount)
	}

	// 6. ContentType is text/html; charset=utf-8.
	if ContentType != "text/html; charset=utf-8" {
		t.Fatalf("ContentType = %q, want %q", ContentType, "text/html; charset=utf-8")
	}
}
