/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/andrewhowdencom/ruf/cmd"
	"github.com/andrewhowdencom/ruf/internal/otel"
)

func main() {
	cmd.Bootstrap()

	shutdown, err := otel.Init()
	if err != nil {
		slog.Error("failed to initialize opentelemetry", "error", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			slog.Error("failed to shutdown opentelemetry", "error", err)
		}
	}()

	cmd.Execute()
}
