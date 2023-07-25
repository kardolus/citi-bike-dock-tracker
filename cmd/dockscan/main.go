package main

import (
	"encoding/json"
	"fmt"
	"github.com/kardolus/citi-bike-dock-tracker/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var (
	GitCommit  string
	GitVersion string
	ServiceURL string
	ids        []string
	exclude    []string
	interval   int
	csv        bool
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

	var cmdTs = &cobra.Command{
		Use:   "ts",
		Short: "Retrieve and display Citibike dock station status with timestamps in JSONL format.",
		Long:  "The 'ts' command retrieves and displays the current status of Citibike dock stations with timestamps in JSON Lines (JSONL) format.",
		RunE:  runTs,
	}

	cmdTs.Flags().StringSliceVar(&ids, "id", []string{}, "Filter dock station status by IDs")
	cmdTs.Flags().IntVar(&interval, "interval", 60, "Set the time interval (in seconds) between fetching station status updates")
	cmdTs.Flags().BoolVar(&csv, "csv", false, "Output station status in CSV format")
	cmdTs.Flags().StringSliceVar(&exclude, "exclude", []string{}, "Exclude specific columns from the CSV output")

	rootCmd.AddCommand(cmdTs)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runInfo(cmd *cobra.Command, args []string) error {
	builder := client.NewClientBuilder()

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

func runTs(cmd *cobra.Command, args []string) error {
	builder := client.NewClientBuilder()

	if ServiceURL != "" {
		builder = builder.WithServiceURL(ServiceURL)
	}

	if len(ids) > 0 {
		builder = builder.WithIDFilter(ids)
	}

	if interval > 0 {
		builder = builder.WithInterval(interval)
	}

	c, err := builder.Build()
	if err != nil {
		return err
	}

	if csv {
		c.PrintStationDataCSV(exclude)
	} else {
		c.PrintStationDataJSONL()
	}

	return nil
}

func runVersion(cmd *cobra.Command, args []string) error {
	fmt.Printf("dockscan version %s (commit %s)\n", GitVersion, GitCommit)

	return nil
}
