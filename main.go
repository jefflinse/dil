package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func main() {
	var cmd = &cobra.Command{
		Use:   "dil [package]",
		Short: "DIL: Dependency Injection Linter",
		Run:   runLinter,
		Args:  cobra.ExactArgs(1),
	}

	setupConfig()

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func setupConfig() {
	// .dil.yml
	viper.SetConfigName(".dil")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("getting user home directory: %v", err)
	}

	viper.AddConfigPath(homeDir)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %v", err)
	}

	log.Printf("using config file: %s\n", viper.ConfigFileUsed())

	// set defaults
	if !viper.IsSet("allow_packages") {
		viper.Set("allow_packages", []string{})
	}

	if !viper.IsSet("exclude_funcs") {
		viper.Set("exclude_funcs", []string{})
	}
}
