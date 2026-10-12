package decision

import (
	"testing"

	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// These tests exercise the ADR-001 precedence chain directly, including
// the conflicting-permission cases REQ-AUTH-008 explicitly requires
// ("conflicting role/user permissions are covered by tests") that no
// end-to-end test reaches, because the committed system-config.json only
// ever defines one role-level ALLOW permission. Each test builds its own
// minimal in-memory Config rather than loading the file, so the
// precedence logic is verified independently of that specific
// configuration.

const (
	testDoor      = "door-1"
	testDirection = "PUBLIC_TO_EMPLOYEE"
)

func baseConfig() *config.Config {
	return &config.Config{
		Users: []config.User{
			{UserID: "user-1", Roles: []string{"employee"}},
		},
		AccessCards: []config.AccessCard{
			{CardID: "card-1", UserID: "user-1", Status: config.CardStatusActive},
		},
	}
}

func permission(subjectType, subjectID, effect string) config.Permission {
	return config.Permission{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		DoorID:      testDoor,
		Direction:   testDirection,
		Effect:      effect,
		Enabled:     true,
	}
}

func TestAuthorize_DefaultDeny_NoApplicablePermission(t *testing.T) {
	cfg := baseConfig() // no permissions at all

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny {
		t.Fatalf("result = %q, want DENY (REQ-AUTH-007 fail-closed)", got.result)
	}
	if got.reason != contracts.ReasonNoPermission {
		t.Fatalf("reason = %q, want %q", got.reason, contracts.ReasonNoPermission)
	}
}

func TestAuthorize_UnknownCard_Denied(t *testing.T) {
	cfg := baseConfig()

	got := authorize(cfg, "card-does-not-exist", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonCardUnknown {
		t.Fatalf("got %+v, want DENY/%s (REQ-AUTH-006)", got, contracts.ReasonCardUnknown)
	}
}

func TestAuthorize_DisabledCard_Denied(t *testing.T) {
	cfg := baseConfig()
	cfg.AccessCards[0].Status = "DISABLED"
	cfg.Permissions = []config.Permission{permission(config.SubjectRole, "employee", config.EffectAllow)}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonCardDisabled {
		t.Fatalf("got %+v, want DENY/%s (REQ-AUTH-006) even though a role ALLOW exists", got, contracts.ReasonCardDisabled)
	}
}

func TestAuthorize_CardWithNoActiveUser_Denied(t *testing.T) {
	cfg := baseConfig()
	cfg.AccessCards[0].UserID = "no-such-user"

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonCardUnknown {
		t.Fatalf("got %+v, want DENY/%s (design-specification.md section 5.2)", got, contracts.ReasonCardUnknown)
	}
}

func TestAuthorize_RoleAllow_Allowed(t *testing.T) {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{permission(config.SubjectRole, "employee", config.EffectAllow)}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultAllow || got.reason != contracts.ReasonRoleAllow {
		t.Fatalf("got %+v, want ALLOW/%s", got, contracts.ReasonRoleAllow)
	}
}

func TestAuthorize_RoleDeny_Denied(t *testing.T) {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{permission(config.SubjectRole, "employee", config.EffectDeny)}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonRoleDeny {
		t.Fatalf("got %+v, want DENY/%s", got, contracts.ReasonRoleDeny)
	}
}

// --- ADR-001 precedence conflicts -----------------------------------------

func TestAuthorize_Precedence_UserDenyOverridesRoleAllow(t *testing.T) {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{
		permission(config.SubjectRole, "employee", config.EffectAllow),
		permission(config.SubjectUser, "user-1", config.EffectDeny),
	}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonUserDeny {
		t.Fatalf("got %+v, want DENY/%s: a user-specific DENY must override a role ALLOW (ADR-001)", got, contracts.ReasonUserDeny)
	}
}

func TestAuthorize_Precedence_UserAllowOverridesRoleDeny(t *testing.T) {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{
		permission(config.SubjectRole, "employee", config.EffectDeny),
		permission(config.SubjectUser, "user-1", config.EffectAllow),
	}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultAllow || got.reason != contracts.ReasonUserAllow {
		t.Fatalf("got %+v, want ALLOW/%s: a user-specific ALLOW must override a role DENY (ADR-001)", got, contracts.ReasonUserAllow)
	}
}

func TestAuthorize_Precedence_UserLevelDenyBeatsUserLevelAllow(t *testing.T) {
	// "Within the same subject level, DENY takes precedence over ALLOW"
	// (ADR-001, design-specification.md section 5.4) — two conflicting
	// user-specific permissions for the same subject.
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{
		permission(config.SubjectUser, "user-1", config.EffectAllow),
		permission(config.SubjectUser, "user-1", config.EffectDeny),
	}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonUserDeny {
		t.Fatalf("got %+v, want DENY/%s", got, contracts.ReasonUserDeny)
	}
}

func TestAuthorize_Precedence_RoleLevelDenyBeatsRoleLevelAllow(t *testing.T) {
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{
		permission(config.SubjectRole, "employee", config.EffectAllow),
		permission(config.SubjectRole, "employee", config.EffectDeny),
	}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonRoleDeny {
		t.Fatalf("got %+v, want DENY/%s", got, contracts.ReasonRoleDeny)
	}
}

func TestAuthorize_PermissionForDifferentDoor_Ignored(t *testing.T) {
	cfg := baseConfig()
	p := permission(config.SubjectRole, "employee", config.EffectAllow)
	p.DoorID = "door-other"
	cfg.Permissions = []config.Permission{p}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonNoPermission {
		t.Fatalf("got %+v, want DENY/%s: a permission for a different door must not apply", got, contracts.ReasonNoPermission)
	}
}

func TestAuthorize_DisabledPermission_Ignored(t *testing.T) {
	cfg := baseConfig()
	p := permission(config.SubjectRole, "employee", config.EffectAllow)
	p.Enabled = false
	cfg.Permissions = []config.Permission{p}

	got := authorize(cfg, "card-1", testDoor, testDirection)

	if got.result != contracts.AuthResultDeny || got.reason != contracts.ReasonNoPermission {
		t.Fatalf("got %+v, want DENY/%s: a disabled permission must not apply", got, contracts.ReasonNoPermission)
	}
}

func TestAuthorize_Deterministic(t *testing.T) {
	// design-specification.md section 5: "Given the same ... the
	// authorization service shall produce the same result."
	cfg := baseConfig()
	cfg.Permissions = []config.Permission{permission(config.SubjectRole, "employee", config.EffectAllow)}

	first := authorize(cfg, "card-1", testDoor, testDirection)
	for i := 0; i < 5; i++ {
		got := authorize(cfg, "card-1", testDoor, testDirection)
		if got != first {
			t.Fatalf("run %d: got %+v, want %+v (not deterministic)", i, got, first)
		}
	}
}
