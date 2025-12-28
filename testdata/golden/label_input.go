// Package main implements the bd CLI label management commands.
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/beads/internal/rpc"
	"github.com/steveyegge/beads/internal/types"
	"github.com/steveyegge/beads/internal/ui"
)

var labelCmd = &cobra.Command{
	Use:     "label",
	GroupID: "issues",
	Short:   "Manage issue labels",
}

// resolveIDs resolves a list of partial IDs using the unified handler.
func resolveIDs(partialIDs []string) []string {
	resolvedIDs := make([]string, 0, len(partialIDs))
	for _, id := range partialIDs {
		resolveResp, err := handler.ResolveID(&rpc.ResolveIDArgs{ID: id})
		if err != nil {
			WarnError("resolving %s: %v", id, err)
			continue
		}
		var fullID string
		if err := json.Unmarshal(resolveResp.Data, &fullID); err != nil {
			WarnError("unmarshaling resolved ID: %v", err)
			continue
		}
		resolvedIDs = append(resolvedIDs, fullID)
	}
	return resolvedIDs
}
func parseLabelArgs(args []string) (issueIDs []string, label string) {
	label = args[len(args)-1]
	issueIDs = args[:len(args)-1]
	return
}

//nolint:dupl // labelAddCmd and labelRemoveCmd are similar but serve different operations
var labelAddCmd = &cobra.Command{
	Use:   "add [issue-id...] [label]",
	Short: "Add a label to one or more issues",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("label add")
		issueIDs, label := parseLabelArgs(args)

		// Protect reserved label namespaces (bd-eijl)
		// provides:* labels can only be added via 'bd ship' command
		if strings.HasPrefix(label, "provides:") {
			FatalErrorRespectJSON("'provides:' labels are reserved for cross-project capabilities. Hint: use 'bd ship %s' instead", strings.TrimPrefix(label, "provides:"))
		}

		// Resolve partial IDs using unified handler
		issueIDs = resolveIDs(issueIDs)

		// Process each issue
		results := []map[string]interface{}{}
		for _, issueID := range issueIDs {
			_, err := handler.AddLabel(&rpc.LabelAddArgs{ID: issueID, Label: label})
			if err != nil {
				WarnError("adding label to %s: %v", issueID, err)
				continue
			}
			if jsonOutput {
				results = append(results, map[string]interface{}{
					"status":   "added",
					"issue_id": issueID,
					"label":    label,
				})
			} else {
				fmt.Printf("%s Added label '%s' to %s\n", ui.RenderPass("✓"), label, issueID)
			}
		}
		if jsonOutput && len(results) > 0 {
			outputJSON(results)
		}
	},
}

//nolint:dupl // labelRemoveCmd and labelAddCmd are similar but serve different operations
var labelRemoveCmd = &cobra.Command{
	Use:   "remove [issue-id...] [label]",
	Short: "Remove a label from one or more issues",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("label remove")
		issueIDs, label := parseLabelArgs(args)

		// Resolve partial IDs using unified handler
		issueIDs = resolveIDs(issueIDs)

		// Process each issue
		results := []map[string]interface{}{}
		for _, issueID := range issueIDs {
			_, err := handler.RemoveLabel(&rpc.LabelRemoveArgs{ID: issueID, Label: label})
			if err != nil {
				WarnError("removing label from %s: %v", issueID, err)
				continue
			}
			if jsonOutput {
				results = append(results, map[string]interface{}{
					"status":   "removed",
					"issue_id": issueID,
					"label":    label,
				})
			} else {
				fmt.Printf("%s Removed label '%s' from %s\n", ui.RenderPass("✓"), label, issueID)
			}
		}
		if jsonOutput && len(results) > 0 {
			outputJSON(results)
		}
	},
}
var labelListCmd = &cobra.Command{
	Use:   "list [issue-id]",
	Short: "List labels for an issue",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Resolve partial ID using unified handler
		var issueID string
		resolveResp, err := handler.ResolveID(&rpc.ResolveIDArgs{ID: args[0]})
		if err != nil {
			FatalErrorRespectJSON("resolving issue ID %s: %v", args[0], err)
		}
		if err := json.Unmarshal(resolveResp.Data, &issueID); err != nil {
			FatalErrorRespectJSON("unmarshaling resolved ID: %v", err)
		}

		// Get issue to retrieve labels
		resp, err := handler.Show(&rpc.ShowArgs{ID: issueID})
		if err != nil {
			FatalErrorRespectJSON("%v", err)
		}
		var issue types.Issue
		if err := json.Unmarshal(resp.Data, &issue); err != nil {
			FatalErrorRespectJSON("parsing response: %v", err)
		}
		labels := issue.Labels

		if jsonOutput {
			// Always output array, even if empty
			if labels == nil {
				labels = []string{}
			}
			outputJSON(labels)
			return
		}
		if len(labels) == 0 {
			fmt.Printf("\n%s has no labels\n", issueID)
			return
		}
		fmt.Printf("\n%s Labels for %s:\n", ui.RenderAccent("🏷"), issueID)
		for _, label := range labels {
			fmt.Printf("  - %s\n", label)
		}
		fmt.Println()
	},
}
var labelListAllCmd = &cobra.Command{
	Use:   "list-all",
	Short: "List all unique labels in the database",
	Run: func(cmd *cobra.Command, args []string) {
		// Get all issues using unified handler
		resp, err := handler.List(&rpc.ListArgs{})
		if err != nil {
			FatalErrorRespectJSON("%v", err)
		}
		var issues []*types.Issue
		if err := json.Unmarshal(resp.Data, &issues); err != nil {
			FatalErrorRespectJSON("parsing response: %v", err)
		}

		// Collect unique labels with counts
		labelCounts := make(map[string]int)
		for _, issue := range issues {
			for _, label := range issue.Labels {
				labelCounts[label]++
			}
		}
		if len(labelCounts) == 0 {
			if jsonOutput {
				outputJSON([]string{})
			} else {
				fmt.Println("\nNo labels found in database")
			}
			return
		}
		// Sort labels alphabetically
		labels := make([]string, 0, len(labelCounts))
		for label := range labelCounts {
			labels = append(labels, label)
		}
		sort.Strings(labels)
		if jsonOutput {
			// Output as array of {label, count} objects
			type labelInfo struct {
				Label string `json:"label"`
				Count int    `json:"count"`
			}
			result := make([]labelInfo, 0, len(labels))
			for _, label := range labels {
				result = append(result, labelInfo{
					Label: label,
					Count: labelCounts[label],
				})
			}
			outputJSON(result)
			return
		}
		fmt.Printf("\n%s All labels (%d unique):\n", ui.RenderAccent("🏷"), len(labels))
		// Find longest label for alignment
		maxLen := 0
		for _, label := range labels {
			if len(label) > maxLen {
				maxLen = len(label)
			}
		}
		for _, label := range labels {
			padding := strings.Repeat(" ", maxLen-len(label))
			fmt.Printf("  %s%s  (%d issues)\n", label, padding, labelCounts[label])
		}
		fmt.Println()
	},
}

func init() {
	labelCmd.AddCommand(labelAddCmd)
	labelCmd.AddCommand(labelRemoveCmd)
	labelCmd.AddCommand(labelListCmd)
	labelCmd.AddCommand(labelListAllCmd)
	rootCmd.AddCommand(labelCmd)
}
