package state

import (
	"encoding/json"
	"net/http"
)

// https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest
// https://www.w3.org/TR/appmanifest/
var manifestData = map[string]any{
	"name":        "tangled",
	"description": "tightly-knit social coding.",
	"icons": []map[string]string{
		{
			"src":   "/static/logos/dolly.svg",
			"sizes": "144x144",
		},
	},
	"start_url":        "/",
	"id":               "https://tangled.org",
	"display":          "standalone",
	"background_color": "#111827",
	"theme_color":      "#111827",
}

func (p *State) WebAppManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	json.NewEncoder(w).Encode(manifestData)
}
