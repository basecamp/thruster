package main

import (
	"log/slog"
	"os"

	"github.com/basecamp/thruster/internal"
)

func setLogger() {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func main() {
	setLogger()

	config, err := internal.NewConfig()
	if err != nil {
		panic(err)
	}

	service := internal.NewService(config)
	os.Exit(service.Run())
}
