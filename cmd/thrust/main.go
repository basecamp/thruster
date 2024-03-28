package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/basecamp/thruster/internal"
)

func setLogger(level slog.Level) {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}

func main() {
	setLogger(slog.LevelWarn)

	config, err := internal.NewConfig()
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	setLogger(config.LogLevel)

	service := internal.NewService(config)
	os.Exit(service.Run())
}
