package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"silobang/internal/constants"
)

// ValidateConstraintsJSON validates that the constraints JSON matches the expected
// schema for the given action. Uses strict parsing that rejects unknown fields to
// prevent silent misconfiguration (e.g., typos like "daly_count_limit").
// Returns nil if constraints are nil/empty (unrestricted), or if JSON is valid.
func ValidateConstraintsJSON(action string, constraintsJSON *string) error {
	if constraintsJSON == nil || *constraintsJSON == "" || *constraintsJSON == "{}" || *constraintsJSON == "null" {
		return nil
	}

	var target interface{}
	switch action {
	case constants.AuthActionUpload:
		target = &UploadConstraints{}
	case constants.AuthActionDownload:
		target = &DownloadConstraints{}
	case constants.AuthActionQuery:
		target = &QueryConstraints{}
	case constants.AuthActionManageUsers:
		target = &ManageUsersConstraints{}
	case constants.AuthActionManageTopics:
		target = &ManageTopicsConstraints{}
	case constants.AuthActionMetadata:
		target = &MetadataConstraints{}
	case constants.AuthActionBulkDownload:
		target = &BulkDownloadConstraints{}
	case constants.AuthActionViewAudit:
		target = &ViewAuditConstraints{}
	case constants.AuthActionVerify:
		target = &VerifyConstraints{}
	case constants.AuthActionManageConfig:
		return fmt.Errorf("action %q does not support constraints", action)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	decoder := json.NewDecoder(strings.NewReader(*constraintsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid constraints for action %q: %w", action, err)
	}

	return nil
}
