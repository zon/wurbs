package main

import (
	"github.com/alecthomas/kong"
	"github.com/zon/chat/wurbctl/migrate"
	"github.com/zon/chat/wurbctl/set"
)

var cli struct {
	Migrate migrate.Cmd `cmd:"" help:"Run database migrations."`
	Set     set.Cmd     `cmd:"" help:"Configure Wurbs settings."`
}

func main() {
	ktx := kong.Parse(&cli,
		kong.Name("wurbctl"),
		kong.Description("CLI for managing Wurbs configuration and database."),
	)
	ktx.FatalIfErrorf(ktx.Run())
}
