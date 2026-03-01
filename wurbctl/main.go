package main

import (
	"github.com/alecthomas/kong"
	"github.com/zon/chat/wurbctl/migrate"
)

var cli struct {
	Migrate migrate.Cmd `cmd:"" help:"Run database migrations."`
}

func main() {
	ktx := kong.Parse(&cli,
		kong.Name("wurbctl"),
		kong.Description("CLI for managing Wurbs configuration and database."),
	)
	ktx.FatalIfErrorf(ktx.Run())
}
