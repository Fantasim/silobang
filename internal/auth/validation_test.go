package auth

import (
	"testing"

	"silobang/internal/constants"
)

func TestValidateConstraintsJSON_NilIsValid(t *testing.T) {
	err := ValidateConstraintsJSON(constants.AuthActionUpload, nil)
	if err != nil {
		t.Errorf("expected nil for nil constraints, got: %v", err)
	}
}

func TestValidateConstraintsJSON_EmptyStringIsValid(t *testing.T) {
	empty := ""
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &empty)
	if err != nil {
		t.Errorf("expected nil for empty string, got: %v", err)
	}
}

func TestValidateConstraintsJSON_EmptyObjectIsValid(t *testing.T) {
	obj := "{}"
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &obj)
	if err != nil {
		t.Errorf("expected nil for empty object, got: %v", err)
	}
}

func TestValidateConstraintsJSON_NullStringIsValid(t *testing.T) {
	null := "null"
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &null)
	if err != nil {
		t.Errorf("expected nil for 'null' string, got: %v", err)
	}
}

func TestValidateConstraintsJSON_ValidUploadConstraints(t *testing.T) {
	valid := `{"allowed_extensions":["png","jpg"],"max_file_size_bytes":1048576,"daily_count_limit":100,"daily_volume_bytes":10737418240,"allowed_topics":["images"]}`
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &valid)
	if err != nil {
		t.Errorf("expected nil for valid upload constraints, got: %v", err)
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldUpload(t *testing.T) {
	// Typo: "daly_count_limit" instead of "daily_count_limit"
	typo := `{"daly_count_limit":10}`
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &typo)
	if err == nil {
		t.Error("expected error for unknown field 'daly_count_limit', got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldDownload(t *testing.T) {
	typo := `{"daly_volume_bytes":999}`
	err := ValidateConstraintsJSON(constants.AuthActionDownload, &typo)
	if err == nil {
		t.Error("expected error for unknown field in download constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldQuery(t *testing.T) {
	typo := `{"allowed_preset":["count"]}`
	err := ValidateConstraintsJSON(constants.AuthActionQuery, &typo)
	if err == nil {
		t.Error("expected error for unknown field in query constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldManageUsers(t *testing.T) {
	typo := `{"can_creat":true}`
	err := ValidateConstraintsJSON(constants.AuthActionManageUsers, &typo)
	if err == nil {
		t.Error("expected error for unknown field in manage_users constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldManageTopics(t *testing.T) {
	typo := `{"can_delet":true}`
	err := ValidateConstraintsJSON(constants.AuthActionManageTopics, &typo)
	if err == nil {
		t.Error("expected error for unknown field in manage_topics constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldMetadata(t *testing.T) {
	typo := `{"daily_count_limt":5}`
	err := ValidateConstraintsJSON(constants.AuthActionMetadata, &typo)
	if err == nil {
		t.Error("expected error for unknown field in metadata constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldBulkDownload(t *testing.T) {
	typo := `{"max_assets_per_reqest":50}`
	err := ValidateConstraintsJSON(constants.AuthActionBulkDownload, &typo)
	if err == nil {
		t.Error("expected error for unknown field in bulk_download constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldViewAudit(t *testing.T) {
	typo := `{"can_view_al":true}`
	err := ValidateConstraintsJSON(constants.AuthActionViewAudit, &typo)
	if err == nil {
		t.Error("expected error for unknown field in view_audit constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsUnknownFieldVerify(t *testing.T) {
	typo := `{"daily_count_limt":10}`
	err := ValidateConstraintsJSON(constants.AuthActionVerify, &typo)
	if err == nil {
		t.Error("expected error for unknown field in verify constraints, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsConstraintsOnManageConfig(t *testing.T) {
	any := `{"some_field":true}`
	err := ValidateConstraintsJSON(constants.AuthActionManageConfig, &any)
	if err == nil {
		t.Error("expected error for constraints on manage_config, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsMalformedJSON(t *testing.T) {
	malformed := `{not json`
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &malformed)
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
}

func TestValidateConstraintsJSON_RejectsWrongTypeValue(t *testing.T) {
	wrongType := `{"daily_count_limit":"not_a_number"}`
	err := ValidateConstraintsJSON(constants.AuthActionUpload, &wrongType)
	if err == nil {
		t.Error("expected error for wrong type value, got nil")
	}
}

func TestValidateConstraintsJSON_ValidManageUsersWithCanGrantActions(t *testing.T) {
	valid := `{"can_create":true,"can_edit":true,"can_grant_actions":["upload","download"],"escalation_allowed":false}`
	err := ValidateConstraintsJSON(constants.AuthActionManageUsers, &valid)
	if err != nil {
		t.Errorf("expected nil for valid manage_users constraints, got: %v", err)
	}
}

func TestValidateConstraintsJSON_ValidAllActionTypes(t *testing.T) {
	cases := []struct {
		action string
		json   string
	}{
		{constants.AuthActionUpload, `{"allowed_extensions":["bin"]}`},
		{constants.AuthActionDownload, `{"daily_count_limit":50}`},
		{constants.AuthActionQuery, `{"allowed_presets":["count"]}`},
		{constants.AuthActionManageUsers, `{"can_create":true}`},
		{constants.AuthActionManageTopics, `{"can_create":true}`},
		{constants.AuthActionMetadata, `{"daily_count_limit":10}`},
		{constants.AuthActionBulkDownload, `{"max_assets_per_request":50}`},
		{constants.AuthActionViewAudit, `{"can_view_all":true}`},
		{constants.AuthActionVerify, `{"daily_count_limit":20}`},
	}

	for _, tc := range cases {
		json := tc.json
		err := ValidateConstraintsJSON(tc.action, &json)
		if err != nil {
			t.Errorf("action=%s: expected nil for valid constraints, got: %v", tc.action, err)
		}
	}
}

func TestValidateConstraintsJSON_UnknownAction(t *testing.T) {
	any := `{"field":true}`
	err := ValidateConstraintsJSON("nonexistent_action", &any)
	if err == nil {
		t.Error("expected error for unknown action, got nil")
	}
}
