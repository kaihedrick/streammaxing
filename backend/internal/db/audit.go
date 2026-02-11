package db

import (
	"context"
	"encoding/json"
	"log"
)

// InsertAuditLog records a sensitive operation in the audit_log table.
// It is fire-and-forget: errors are logged but do not propagate to callers,
// so audit failures never block the main request path.
func InsertAuditLog(ctx context.Context, userID, action, resourceType, resourceID string, details map[string]interface{}, ipAddress string, success bool) {
	if Pool == nil {
		return
	}

	detailsJSON, err := json.Marshal(details)
	if err != nil {
		log.Printf("[AUDIT_WARN] Failed to marshal details: %v", err)
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_log (user_id, action, resource_type, resource_id, details, ip_address, success)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = Pool.Exec(ctx, query, userID, action, resourceType, resourceID, detailsJSON, ipAddress, success)
	if err != nil {
		log.Printf("[AUDIT_WARN] Failed to insert audit log: %v", err)
	}
}
