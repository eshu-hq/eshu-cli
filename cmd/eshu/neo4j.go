// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// This file carries the Tier-1 neo4j connection-guidance verb. It is split
// out of the config family (which stays a Tier-2 E3 STUB in this population)
// because the guidance touches no database, filesystem lifecycle, or
// deployment asset: it only prints where to set connection values.
func init() {
	// neo4j setup
	neo4jCmd := &cobra.Command{
		Use:   "neo4j",
		Short: "Neo4j database configuration",
	}
	neo4jCmd.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Configure Neo4j database connection",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Neo4j Database Setup")
			fmt.Println("Set NEO4J_URI, NEO4J_USERNAME, NEO4J_PASSWORD environment variables")
			fmt.Println("e.g. export NEO4J_URI=bolt://localhost:7687 (persistent config lives in the Eshu core binary)")
			return nil
		},
	})
	rootCmd.AddCommand(neo4jCmd)

	// Shortcut: eshu n -> neo4j setup
	nAlias := &cobra.Command{
		Use:   "n",
		Short: "Shortcut for 'eshu neo4j setup'",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Neo4j Database Setup")
			fmt.Println("Set NEO4J_URI, NEO4J_USERNAME, NEO4J_PASSWORD environment variables")
			fmt.Println("e.g. export NEO4J_URI=bolt://localhost:7687 (persistent config lives in the Eshu core binary)")
			return nil
		},
	}
	rootCmd.AddCommand(nAlias)
}
