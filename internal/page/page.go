// Package page holds the one hand-written HTML file fleetgauge serves at /.
//
// The page is embedded into the binary at build time, so the single static
// binary really is the whole deployment — there is no asset directory to ship
// alongside it and no path to get wrong. SPEC.md's non-goals bind this file:
// inline CSS and JS only, no framework, no build step, no CDN, no web fonts,
// and no external request of any kind at runtime.
package page

import (
	_ "embed"
	"strconv"
)

//go:embed index.html
var html []byte

// HTML returns the page bytes. It returns the shared slice rather than a copy:
// handlers write it and never modify it, and copying 20 KB per request to
// defend against a mistake nobody has made is not a trade worth making.
func HTML() []byte { return html }

// ContentType is what the / handler must send with HTML.
const ContentType = "text/html; charset=utf-8"

// ContentLength is the page size as a decimal string, for the Content-Length
// header.
func ContentLength() string { return strconv.Itoa(len(html)) }
