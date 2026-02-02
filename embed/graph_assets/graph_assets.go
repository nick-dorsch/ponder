package graph_assets

import "embed"

// Assets holds the static files for the dashboard.
//
//go:embed index.html graph.js favicon.svg
var Assets embed.FS
