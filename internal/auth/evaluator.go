package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"silobang/internal/constants"
	"silobang/internal/logger"
)

// PolicyEvaluator evaluates authorization policies for requests.
// It implements the 3-phase evaluation: grant check → constraint check → quota check.
type PolicyEvaluator struct {
	store  *Store
	logger *logger.Logger
}

// NewPolicyEvaluator creates a new policy evaluator.
func NewPolicyEvaluator(store *Store, log *logger.Logger) *PolicyEvaluator {
	return &PolicyEvaluator{store: store, logger: log}
}

// Evaluate checks if the given identity is authorized for the action context.
// Returns a PolicyResult with the decision, reason, and matched grant.
func (e *PolicyEvaluator) Evaluate(identity *Identity, ctx *ActionContext) *PolicyResult {
	if identity == nil || identity.User == nil {
		return denied(constants.ErrCodeAuthRequired, "authentication required")
	}

	if !identity.User.IsActive {
		return denied(constants.ErrCodeAuthUserDisabled, "user account is disabled")
	}

	// Phase 1: Grant check — find active grants for this action
	var matchingGrants []Grant
	for _, g := range identity.Grants {
		if g.Action == ctx.Action && g.IsActive {
			matchingGrants = append(matchingGrants, g)
		}
	}

	if len(matchingGrants) == 0 {
		e.logger.Debug("Auth denied: user=%s has no grants for action=%s", identity.User.Username, ctx.Action)
		return denied(constants.ErrCodeAuthForbidden,
			fmt.Sprintf("no permission for action: %s", ctx.Action))
	}

	// Phase 2+3: Evaluate each matching grant (first passing grant wins)
	var lastReason string
	var lastCode string
	for i := range matchingGrants {
		grant := &matchingGrants[i]
		result := e.evaluateGrant(identity, grant, ctx)
		if result.Allowed {
			e.logger.Debug("Auth allowed: user=%s action=%s grant_id=%d",
				identity.User.Username, ctx.Action, grant.ID)
			return result
		}
		lastReason = result.Reason
		lastCode = result.DeniedCode
	}

	// All grants had constraint/quota violations
	e.logger.Debug("Auth denied: user=%s action=%s reason=%s",
		identity.User.Username, ctx.Action, lastReason)
	return denied(lastCode, lastReason)
}

// evaluateGrant checks constraints and quotas for a single grant.
func (e *PolicyEvaluator) evaluateGrant(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	// No constraints = unrestricted (but still check quotas if any exist in constraints)
	if grant.ConstraintsJSON == nil || *grant.ConstraintsJSON == "" || *grant.ConstraintsJSON == "{}" || *grant.ConstraintsJSON == "null" {
		return allowed(grant)
	}

	switch ctx.Action {
	case constants.AuthActionUpload:
		return e.evaluateUpload(identity, grant, ctx)
	case constants.AuthActionDownload:
		return e.evaluateDownload(identity, grant, ctx)
	case constants.AuthActionQuery:
		return e.evaluateQuery(identity, grant, ctx)
	case constants.AuthActionManageUsers:
		return e.evaluateManageUsers(grant, ctx)
	case constants.AuthActionManageTopics:
		return e.evaluateManageTopics(grant, ctx)
	case constants.AuthActionMetadata:
		return e.evaluateMetadata(identity, grant, ctx)
	case constants.AuthActionBulkDownload:
		return e.evaluateBulkDownload(identity, grant, ctx)
	case constants.AuthActionViewAudit:
		return e.evaluateViewAudit(grant, ctx)
	case constants.AuthActionVerify:
		return e.evaluateVerify(identity, grant, ctx)
	default:
		// For actions without specific constraint types (manage_config),
		// having the grant is sufficient
		return allowed(grant)
	}
}

// ============================================================================
// Per-action constraint evaluators
// ============================================================================

func (e *PolicyEvaluator) evaluateUpload(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c UploadConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse upload constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	// Check allowed extensions
	if len(c.AllowedExtensions) > 0 && ctx.Extension != "" {
		if !containsString(c.AllowedExtensions, strings.ToLower(ctx.Extension)) {
			return denied(constants.ErrCodeAuthConstraintViolation,
				fmt.Sprintf("file extension %q not allowed", ctx.Extension))
		}
	}

	// Check max file size
	if c.MaxFileSizeBytes > 0 && ctx.FileSize > 0 && ctx.FileSize > c.MaxFileSizeBytes {
		return denied(constants.ErrCodeAuthConstraintViolation,
			fmt.Sprintf("file size %d exceeds limit %d", ctx.FileSize, c.MaxFileSizeBytes))
	}

	// Check allowed topics
	if result := checkAllowedTopics(c.AllowedTopics, ctx.TopicName); result != nil {
		return result
	}

	// Check daily quotas
	if c.DailyCountLimit > 0 || c.DailyVolumeBytes > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			e.logger.Error("Failed to get quota usage for user=%s action=%s: %v",
				identity.User.Username, ctx.Action, err)
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}

		if c.DailyCountLimit > 0 && usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded,
				fmt.Sprintf("daily upload count limit exceeded (%d/%d)", usage.RequestCount, c.DailyCountLimit))
		}
		if c.DailyVolumeBytes > 0 && ctx.FileSize > 0 && (usage.TotalBytes+ctx.FileSize) > c.DailyVolumeBytes {
			return denied(constants.ErrCodeAuthQuotaExceeded,
				fmt.Sprintf("daily upload volume limit would be exceeded (%d + %d > %d)",
					usage.TotalBytes, ctx.FileSize, c.DailyVolumeBytes))
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateDownload(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c DownloadConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse download constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	if result := checkAllowedTopics(c.AllowedTopics, ctx.TopicName); result != nil {
		return result
	}

	if c.DailyCountLimit > 0 || c.DailyVolumeBytes > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}

		if c.DailyCountLimit > 0 && usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded,
				fmt.Sprintf("daily download count limit exceeded (%d/%d)", usage.RequestCount, c.DailyCountLimit))
		}
		if c.DailyVolumeBytes > 0 && ctx.VolumeBytes > 0 && (usage.TotalBytes+ctx.VolumeBytes) > c.DailyVolumeBytes {
			return denied(constants.ErrCodeAuthQuotaExceeded, "daily download volume limit would be exceeded")
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateQuery(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c QueryConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse query constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	// Check allowed presets
	if len(c.AllowedPresets) > 0 && ctx.PresetName != "" {
		if !containsString(c.AllowedPresets, ctx.PresetName) {
			return denied(constants.ErrCodeAuthConstraintViolation,
				fmt.Sprintf("query preset %q not allowed", ctx.PresetName))
		}
	}

	if result := checkAllowedTopics(c.AllowedTopics, ctx.TopicName); result != nil {
		return result
	}

	if c.DailyCountLimit > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}
		if usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded,
				fmt.Sprintf("daily query count limit exceeded (%d/%d)", usage.RequestCount, c.DailyCountLimit))
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateManageUsers(grant *Grant, ctx *ActionContext) *PolicyResult {
	var c ManageUsersConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse manage_users constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	switch ctx.SubAction {
	case "create":
		if !c.CanCreate {
			return denied(constants.ErrCodeAuthConstraintViolation, "user creation not permitted")
		}
	case "edit":
		if !c.CanEdit {
			return denied(constants.ErrCodeAuthConstraintViolation, "user editing not permitted")
		}
	case "disable":
		if !c.CanDisable {
			return denied(constants.ErrCodeAuthConstraintViolation, "user disabling not permitted")
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateManageTopics(grant *Grant, ctx *ActionContext) *PolicyResult {
	var c ManageTopicsConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse manage_topics constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	if result := checkAllowedTopics(c.AllowedTopics, ctx.TopicName); result != nil {
		return result
	}

	switch ctx.SubAction {
	case "create":
		if !c.CanCreate {
			return denied(constants.ErrCodeAuthConstraintViolation, "topic creation not permitted")
		}
	case "delete":
		if !c.CanDelete {
			return denied(constants.ErrCodeAuthConstraintViolation, "topic deletion not permitted")
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateMetadata(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c MetadataConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse metadata constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	if result := checkAllowedTopics(c.AllowedTopics, ctx.TopicName); result != nil {
		return result
	}

	if c.DailyCountLimit > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}
		if usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded, "daily metadata operation limit exceeded")
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateBulkDownload(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c BulkDownloadConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse bulk_download constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	if c.MaxAssetsPerRequest > 0 && ctx.AssetCount > c.MaxAssetsPerRequest {
		return denied(constants.ErrCodeAuthConstraintViolation,
			fmt.Sprintf("asset count %d exceeds max per request %d", ctx.AssetCount, c.MaxAssetsPerRequest))
	}

	if c.DailyCountLimit > 0 || c.DailyVolumeBytes > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}
		if c.DailyCountLimit > 0 && usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded, "daily bulk download count limit exceeded")
		}
		if c.DailyVolumeBytes > 0 && ctx.VolumeBytes > 0 && (usage.TotalBytes+ctx.VolumeBytes) > c.DailyVolumeBytes {
			return denied(constants.ErrCodeAuthQuotaExceeded, "daily bulk download volume limit would be exceeded")
		}
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateViewAudit(grant *Grant, ctx *ActionContext) *PolicyResult {
	var c ViewAuditConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse view_audit constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	// SubAction "stream" requires can_stream
	if ctx.SubAction == "stream" && !c.CanStream {
		return denied(constants.ErrCodeAuthConstraintViolation, "audit streaming not permitted")
	}

	return allowed(grant)
}

func (e *PolicyEvaluator) evaluateVerify(identity *Identity, grant *Grant, ctx *ActionContext) *PolicyResult {
	var c VerifyConstraints
	if err := json.Unmarshal([]byte(*grant.ConstraintsJSON), &c); err != nil {
		e.logger.Warn("Failed to parse verify constraints for grant %d: %v", grant.ID, err)
		return denied(constants.ErrCodeAuthConstraintViolation, "malformed grant constraints")
	}

	if c.DailyCountLimit > 0 {
		usage, err := e.store.GetTodayUsage(identity.User.ID, ctx.Action)
		if err != nil {
			return denied(constants.ErrCodeAuthQuotaExceeded, "failed to check quota")
		}
		if usage.RequestCount >= c.DailyCountLimit {
			return denied(constants.ErrCodeAuthQuotaExceeded, "daily verification count limit exceeded")
		}
	}

	return allowed(grant)
}

// IncrementQuota increments the daily quota counters after a successful action.
func (e *PolicyEvaluator) IncrementQuota(userID int64, action string, bytes int64) {
	if err := e.store.IncrementQuota(userID, action, 1, bytes); err != nil {
		e.logger.Error("Failed to increment quota for user=%d action=%s: %v", userID, action, err)
	}
}

// ============================================================================
// Helpers
// ============================================================================

func allowed(grant *Grant) *PolicyResult {
	return &PolicyResult{Allowed: true, MatchedGrant: grant}
}

func denied(code, reason string) *PolicyResult {
	return &PolicyResult{Allowed: false, Reason: reason, DeniedCode: code}
}

func containsString(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func checkAllowedTopics(allowedTopics []string, topicName string) *PolicyResult {
	if len(allowedTopics) > 0 && topicName != "" {
		if !containsString(allowedTopics, topicName) {
			return denied(constants.ErrCodeAuthConstraintViolation,
				fmt.Sprintf("topic %q not in allowed list", topicName))
		}
	}
	return nil
}
