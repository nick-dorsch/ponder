package sql

import _ "embed"

// Schema holds the database schema.
//
//go:embed schema.sql
var Schema string
