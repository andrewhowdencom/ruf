/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/andrewhowdencom/ruf/internal/otel"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var logLevel string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ruf",
	Short: "A tool to send calls to different platforms.",
	Long: `A tool to send calls to different platforms.

This application is a CLI tool to send calls to different platforms.
Currently, it supports Slack.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		InitConfig()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/ruf/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	viper.BindPFlag("log.level", rootCmd.PersistentFlags().Lookup("log-level"))

	viper.SetDefault("email.host", "")
	viper.SetDefault("email.port", 587)
	viper.SetDefault("email.username", "")
	viper.SetDefault("email.password", "")
	viper.SetDefault("email.from", "")
	viper.SetDefault("git.tokens", map[string]string{})
	viper.SetDefault("datastore.type", "bbolt")
	viper.SetDefault("datastore.project_id", "")

	rootCmd.PersistentFlags().String("otel-endpoint", "", "OpenTelemetry endpoint")
	viper.BindPFlag("otel.endpoint", rootCmd.PersistentFlags().Lookup("otel-endpoint"))

	viper.SetDefault("otel.endpoint", "")
	viper.SetDefault("otel.headers", map[string]string{})
}

// InitConfig reads in config file and ENV variables if set.
func InitConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find xdg config path and set it for viper if found.
		configPath, err := xdg.ConfigFile("ruf/config.yaml")
		if err == nil {
			// Search config in the XDG config directory with name "config.yaml".
			viper.AddConfigPath(filepath.Dir(configPath))
			viper.SetConfigName(filepath.Base(configPath))
			viper.SetConfigType("yaml")
		}
	}

	viper.SetEnvPrefix("RUF")
	viper.AutomaticEnv() // read in environment variables that match

	configReadErr := viper.ReadInConfig()

	// Initialise the logger
	var programLevel = new(slog.LevelVar)
	switch strings.ToLower(viper.GetString("log.level")) {
	case "debug":
		programLevel.Set(slog.LevelDebug)
	case "warn":
		programLevel.Set(slog.LevelWarn)
	case "error":
		programLevel.Set(slog.LevelError)
	default:
		programLevel.Set(slog.LevelInfo)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: programLevel})
	slog.SetDefault(slog.New(handler))

	if configReadErr != nil {
		if _, ok := configReadErr.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("config file not found")
		} else {
			slog.Warn("could not read config file, using defaults", "error", configReadErr)
		}
	}

	// Initialise OpenTelemetry
	if viper.GetString("otel.endpoint") != "" {
		otelShutdown, err := otel.SetupOTelSDK(context.Background(), viper.GetString("otel.endpoint"), viper.GetStringMapString("otel.headers"))
		if err != nil {
			slog.Error("could not setup OpenTelemetry", "error", err)
			os.Exit(1)
		}
		cobra.OnFinalize(func() {
			if err := otelShutdown(context.Background()); err != nil {
				slog.Error("could not shutdown OpenTelemetry", "error", err)
			}
		})
	}
}
