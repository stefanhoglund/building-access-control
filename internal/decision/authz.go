package decision

import (
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/config"
	"github.com/stefanhoglund-cdot/applied-ai-edge-computing/internal/contracts"
)

// authResult is the internal authorization outcome
// (design-specification.md section 5.5). It is folded into the
// AccessDecision by the caller, not transported on its own.
type authResult struct {
	result string // contracts.AuthResultAllow | AuthResultDeny
	reason string
}

// authorize implements the ADR-001 precedence chain:
//
//  1. invalid credential        -> DENY
//  2. explicit user DENY        -> DENY
//  3. explicit user ALLOW       -> ALLOW
//  4. matching role DENY        -> DENY
//  5. matching role ALLOW       -> ALLOW
//  6. no applicable permission  -> DENY
//
// Permission day/time constraints (REQ-AUTH-005) are not exercised by this
// slice's configuration (docs/data-contracts.md section 7) but the
// matching step is where they would be applied; see
// config.Config.PermissionsFor.
func authorize(cfg *config.Config, cardID, doorID, direction string) authResult {
	card, ok := cfg.FindCard(cardID)
	if !ok {
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonCardUnknown}
	}
	if card.Status != config.CardStatusActive {
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonCardDisabled}
	}

	user, ok := cfg.FindUser(card.UserID)
	if !ok {
		// "not associated with an active User" (design-specification.md
		// section 5.2) is a credential-validation failure.
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonCardUnknown}
	}

	roleSet := make(map[string]bool, len(user.Roles))
	for _, roleID := range user.Roles {
		roleSet[roleID] = true
	}

	permissions := cfg.PermissionsFor(doorID, direction)

	var userDeny, userAllow, roleDeny, roleAllow bool
	for _, p := range permissions {
		switch p.SubjectType {
		case config.SubjectUser:
			if p.SubjectID != user.UserID {
				continue
			}
			if p.Effect == config.EffectDeny {
				userDeny = true
			} else if p.Effect == config.EffectAllow {
				userAllow = true
			}
		case config.SubjectRole:
			if !roleSet[p.SubjectID] {
				continue
			}
			if p.Effect == config.EffectDeny {
				roleDeny = true
			} else if p.Effect == config.EffectAllow {
				roleAllow = true
			}
		}
	}

	switch {
	case userDeny:
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonUserDeny}
	case userAllow:
		return authResult{result: contracts.AuthResultAllow, reason: contracts.ReasonUserAllow}
	case roleDeny:
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonRoleDeny}
	case roleAllow:
		return authResult{result: contracts.AuthResultAllow, reason: contracts.ReasonRoleAllow}
	default:
		return authResult{result: contracts.AuthResultDeny, reason: contracts.ReasonNoPermission}
	}
}
