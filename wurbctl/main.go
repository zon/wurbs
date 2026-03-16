package main

import "github.com/alecthomas/kong"

var cli struct {
	Migrate MigrateCmd `cmd:"" help:"Run database migrations."`
	Set     SetCmd     `cmd:"" help:"Configure Wurbs settings."`
}

func main() {
	ktx := kong.Parse(&cli,
		kong.Name("wurbctl"),
		kong.Description("CLI for managing Wurbs configuration and database."),
	)
	ktx.FatalIfErrorf(ktx.Run())
}
