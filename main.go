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
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
}
