package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/zon/chat/core"
	"github.com/zon/chat/core/pg"
)

var cli struct {
	ConfigDir string `help:"Config directory" default:""`
	Test      bool   `help:"Enable test mode with test users and channels" default:"false"`
	Port      string `help:"Port to host on" default:"8080"`
}

func main() {
	ktx := kong.Parse(&cli)
	ktx.FatalIfErrorf(ktx.Error)

	testMode := cli.Test
	workingDir, _ := os.Getwd()

	configDir := cli.ConfigDir
	if configDir == "" {
		var err error
		configDir, err = core.GetConfigDir(workingDir)
		if err != nil {
			slog.Error("config dir", "error", err)
			os.Exit(1)
		}
	}

	cfg, secrets, err := core.LoadConfig(configDir)
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	postgresSecretPath := filepath.Join(configDir, "postgres.json")
	secret, err := pg.ReadSecret(postgresSecretPath)
	if err != nil {
		slog.Error("read postgres secret", "error", err)
		os.Exit(1)
	}

	db, err := pg.OpenDB(secret)
	if err != nil {
		slog.Error("db", "error", err)
		os.Exit(1)
	}
	core.SetDB(db)

	err = core.AutoMigrate()
	if err != nil {
		slog.Error("auto migrate", "error", err)
		os.Exit(1)
	}

	err = core.ConnectNATS(cfg, secrets)
	if err != nil {
		slog.Error("nats connection failed", "error", err)
		os.Exit(1)
	}

	core.SetTestMode(testMode)

	app := fiber.New()
	app.Use(cors.New())
	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON("ok")
	})

	app.Use(core.AuthMiddleware)

	app.Get("/messages", getMessages)
	app.Post("/messages", postMessage)
	app.Put("/messages/:id", putMessage)
	app.Delete("/messages/:id", deleteMessage)

	err = app.Listen(":" + cli.Port)
	if err != nil {
		slog.Error("Listen failed", "error", err)
	}
}
