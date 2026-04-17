package main

import (
	"context"
	"fmt"
	"os"

	rankedb "github.com/flocko-motion/rankedb/worker"
	"github.com/flocko-motion/ranke-workers/google/contacts"
	"github.com/spf13/cobra"
)

func main() {
	var server string
	var dryRun bool
	var verbose bool

	rootCmd := &cobra.Command{
		Use:   "ranke-worker-google",
		Short: "RankeDB worker for Google Takeout formats",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(server, dryRun, verbose)
		},
	}

	rootCmd.Flags().StringVar(&server, "server", "http://localhost:8000", "RankeDB server URL")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be created without writing to the server")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(server string, dryRun bool, verbose bool) error {
	ctx := context.Background()

	client, err := rankedb.NewClient(server)
	if err != nil {
		return err
	}
	client.DryRun = dryRun
	if verbose {
		client.Verbosity = rankedb.LogDebug
	}

	configID, runID, err := client.StartRun(ctx, rankedb.WorkerConfig{
		Name:    "google/takeout",
		Version: "0.1.0",
	})
	if err != nil {
		return err
	}
	fmt.Printf("Worker config: %s\n", configID)
	fmt.Printf("Run:           %s\n", runID)

	// Find unprocessed bulk nodes
	bulkNodes, err := client.Queue(ctx, rankedb.QueueParams{
		ContentClass: "source",
		ContentType:  "bulk",
	})
	if err != nil {
		return fmt.Errorf("queue: %w", err)
	}

	if len(bulkNodes) == 0 {
		fmt.Println("No unprocessed bulk nodes found.")
		return nil
	}

	fmt.Printf("Found %d unprocessed bulk node(s).\n", len(bulkNodes))

	for i := range bulkNodes {
		bulkNode := &bulkNodes[i]
		fmt.Printf("\nBulk node: %s (%s/%s)\n", bulkNode.Id, bulkNode.ContentClass, bulkNode.ContentType)

		content, err := client.GetNodeContent(ctx, bulkNode.Id)
		if err != nil {
			fmt.Printf("ERROR downloading %s: %v\n", bulkNode.Id, err)
			continue
		}

		// Dispatch to handlers based on archive contents
		if err := contacts.Process(ctx, client, bulkNode.Id, configID, runID, content); err != nil {
			fmt.Printf("ERROR processing contacts in %s: %v\n", bulkNode.Id, err)
		}

		// Future handlers: gmail, calendar, etc.

		if err := client.Done(ctx, bulkNode); err != nil {
			fmt.Printf("ERROR marking done %s: %v\n", bulkNode.Id, err)
		}
	}

	return nil
}
