package main

import (
	"log"

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
	viper.AddConfigPath(".")

	// Try to read the configuration
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, using default values
			log.Println("Config file not found; using default values")
		} else {
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
