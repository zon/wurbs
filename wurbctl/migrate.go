package main

// MigrateCmd is the top-level `migrate` command group.
type MigrateCmd struct {
	DB MigrateDBCmd `cmd:"" name:"db" help:"Apply all pending database migrations."`
}
