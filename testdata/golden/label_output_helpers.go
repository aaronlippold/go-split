// Package main implements the bd CLI label management commands.
package main

import (
	"encoding/json"

	"github.com/steveyegge/beads/internal/rpc"
)

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