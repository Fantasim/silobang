package auth

import (
	"fmt"

	"meshbank/internal/constants"
	"meshbank/internal/logger"
)

// BootstrapResult contains the credentials generated during bootstrap.
// These are shown once and never again.
type BootstrapResult struct {
	Username string
	Password string
	APIKey   string
}

// Bootstrap creates the initial admin user if no users exist.
// Returns the plaintext credentials that must be shown to the operator once.
// Returns nil if users already exist (no bootstrap needed).
func Bootstrap(store *Store, log *logger.Logger) (*BootstrapResult, error) {
	count, err := store.CountUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to check user count: %w", err)
	}

	if count > 0 {
		log.Debug("Auth: %d user(s) exist, skipping bootstrap", count)
		return nil, nil
	}

	log.Info("Auth: no users found, bootstrapping admin account...")

	// Generate credentials
	password, err := GeneratePassword()
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	apiKey, err := GenerateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Hash credentials
	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	apiKeyHash := HashToken(apiKey)
	apiKeyPrefix := ExtractTokenPrefix(apiKey)

	// Create bootstrap user
	user, err := store.CreateBootstrapUser(
		constants.AuthBootstrapUsername,
		"System Administrator",
		passwordHash,
		apiKeyHash,
		apiKeyPrefix,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap user: %w", err)
	}

	// Grant all actions with no constraints (superadmin)
	for _, action := range constants.AllAuthActions {
		_, err := store.CreateGrant(user.ID, action, nil, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create grant for action %s: %w", action, err)
		}
	}

	log.Info("Auth: bootstrap user '%s' created with full permissions (id=%d)",
		constants.AuthBootstrapUsername, user.ID)

	return &BootstrapResult{
		Username: constants.AuthBootstrapUsername,
		Password: password,
		APIKey:   apiKey,
	}, nil
}
