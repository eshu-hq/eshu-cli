// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"github.com/spf13/cobra"
)

// workspaceWatchGuidance is the retained-lifecycle redirect for
// `eshu workspace watch`: the watcher lifecycle stays in the core binary
// (runWatch re-execs the local-host supervisor), so the destination points
// at it instead of running a supervisor it does not own.
const workspaceWatchGuidance = "Watcher lifecycle stays in the core eshu binary. " +
	"Run `eshu workspace watch` in the core binary instead. " +
	"Refs eshu-hq/eshu#8874."

// newWorkspaceCommand builds the `eshu workspace` family: status moves as a
// thin client of the pipeline-status route, plan/sync/index keep their
// removed-command errors, and watch keeps retained-lifecycle guidance. Use,
// Short, Args, and flags stay verbatim with the core family so --help bytes
// match; only the watch body differs (no local supervisor in this binary).
func newWorkspaceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Multi-repository workspace operations",
	}

	planCmd := &cobra.Command{
		Use:   "plan <path>",
		Short: "Removed: use eshu index <path> or eshu admin reindex instead",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspacePlan,
	}
	cmd.AddCommand(planCmd)

	syncCmd := &cobra.Command{
		Use:   "sync <path>",
		Short: "Removed: use eshu index <path> or eshu admin reindex instead",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceSync,
	}
	cmd.AddCommand(syncCmd)

	wsIndexCmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Removed: use eshu index <path> or eshu admin reindex instead",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceIndex,
	}
	cmd.AddCommand(wsIndexCmd)

	statusCmd := &cobra.Command{
		Use:   "status [path]",
		Short: "Show workspace indexing status",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runWorkspaceStatus,
	}
	addRemoteFlags(statusCmd)
	cmd.AddCommand(statusCmd)

	watchCmd := &cobra.Command{
		Use:   "watch <path>",
		Short: "Watch all repositories in a workspace for changes",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkspaceWatch,
	}
	watchCmd.Flags().String("workspace-root", "", "Explicit workspace root for the local Eshu service")
	cmd.AddCommand(watchCmd)

	return cmd
}

func init() {
	rootCmd.AddCommand(newWorkspaceCommand())
}

// workspaceIndexingGuidance replaces the workspace plan, sync, and index
// commands, which posted a path-scoped body the reindex API never honored.
const workspaceIndexingGuidance = "Use `eshu index <path>` to index a local directory tree, or " +
	"`eshu admin reindex` to force every git ingester to re-parse all repositories."

func runWorkspacePlan(cmd *cobra.Command, args []string) error {
	return removedCommandError("eshu workspace plan", workspaceIndexingGuidance)
}

func runWorkspaceSync(cmd *cobra.Command, args []string) error {
	return removedCommandError("eshu workspace sync", workspaceIndexingGuidance)
}

func runWorkspaceIndex(cmd *cobra.Command, args []string) error {
	return removedCommandError("eshu workspace index", workspaceIndexingGuidance)
}

func runWorkspaceStatus(cmd *cobra.Command, args []string) error {
	client := apiClient()
	var result any
	if err := client.Get("/api/v0/status/pipeline", &result); err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runWorkspaceWatch(cmd *cobra.Command, args []string) error {
	return removedCommandError("eshu workspace watch", workspaceWatchGuidance)
}
