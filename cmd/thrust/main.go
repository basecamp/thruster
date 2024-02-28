package main

import (
	"fmt"
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
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	service := internal.NewService(config)
	os.Exit(service.Run())
}
