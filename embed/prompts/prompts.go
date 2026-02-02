package prompts

import _ "embed"

// Header is the static header part of the agent prompt.
//
//go:embed header.md
var Header string

// Footer is the static footer part of the agent prompt.
//
//go:embed footer.md
var Footer string
