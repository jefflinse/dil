package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	var cmd = &cobra.Command{
		Use:   "linter [path]",
		Short: "Linter to detect significant external package usages",
		Run:   runLinter,
		Args:  cobra.ExactArgs(1),
	}

	setupConfig()
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func setupConfig() {
	viper.SetConfigName(".didetect")
	viper.SetConfigType("yml")

	// First, look in the current directory for the config file.
	viper.AddConfigPath(".")

	// If not found in the current directory, then look in the user's home directory.
	if err := viper.ReadInConfig(); err != nil {
		homeDir, errHome := os.UserHomeDir()
		if errHome != nil {
			log.Fatalf("Failed to get home directory, %s", errHome)
		}
		viper.AddConfigPath(homeDir)

		if err = viper.ReadInConfig(); err != nil {
			log.Fatalf("Error reading config file, %s", err)
		}
	}

	// Set defaults
	// If the configuration values are not set in the config file or the config file is missing,
	// these defaults will be used.
	if !viper.IsSet("allow_packages") {
		viper.Set("allow_packages", []string{})
	}
	if !viper.IsSet("exclude_funcs") {
		viper.Set("exclude_funcs", []string{})
	}
}
