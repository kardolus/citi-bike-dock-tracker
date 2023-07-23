package main

import (
	"encoding/json"
	"fmt"
	"github.com/kardolus/citi-bike-dock-tracker/client"
	"github.com/kardolus/citi-bike-dock-tracker/http"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var (
	GitCommit  string
	GitVersion string
	ServiceURL string
	ids        []string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "dockscan",
		Short: "A CLI for tracking Citibike dock station status.",
		Long: "dockscan is a Command Line Interface (CLI) application " +
			"built to track the status of Citibike dock stations. " +
			"It retrieves data from the Citibike dock stations status API " +
			"and maps station IDs to human-readable names using the station information API. ",
	}

	viper.AutomaticEnv()

	var cmdInfo = &cobra.Command{
		Use:   "info",
		Short: "Retrieve and display Citibike dock station status.",
		Long:  "The 'info' command retrieves and displays the current status of Citibike dock stations.",
		RunE:  runInfo,
	}

	cmdInfo.Flags().StringSliceVar(&ids, "id", []string{}, "Filter dock station status by IDs")

	rootCmd.AddCommand(cmdInfo)

	var cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Displays the version of dockscan.",
		Long:  "The 'version' command displays the current version of the dockscan CLI tool.",
		RunE:  runVersion,
	}
	rootCmd.AddCommand(cmdVersion)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runInfo(cmd *cobra.Command, args []string) error {
	builder := client.NewClientBuilder(http.New(), client.RealTime{})

	if ServiceURL != "" {
		builder = builder.WithServiceURL(ServiceURL)
	}

	if len(ids) > 0 {
		builder = builder.WithIDFilter(ids)
	}

	c, err := builder.Build()

	if err != nil {
		return err
	}

	data, err := c.ParseStationData()
	if err != nil {
		return err
	}

	result, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	fmt.Println(string(result))

	return nil
}

func runVersion(cmd *cobra.Command, args []string) error {
	fmt.Printf("dockscan version %s (commit %s)\n", GitVersion, GitCommit)

	return nil
}
