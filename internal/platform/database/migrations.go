package database

import _ "embed"

//go:embed migrations.sql
var MigrationsSQL string
