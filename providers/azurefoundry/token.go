package azurefoundry

import (
	"context"
	"time"
)

// DefaultScope is the OAuth2 scope for Azure Cognitive Services.
// This is used when acquiring tokens via Entra ID authentication.
const DefaultScope = "https://cognitiveservices.azure.com/.default"

// TokenCredential provides access tokens for Azure Entra ID authentication.
// This interface is designed to be compatible with azure-sdk-for-go's azcore.TokenCredential,
// allowing direct use of Azure SDK credentials:
//
//	cred, err := azidentity.NewDefaultAzureCredential(nil)
//	provider := azurefoundry.NewWithCredential(endpoint, cred)
//
// Users can also implement this interface directly for custom token providers.
type TokenCredential interface {
	// GetToken returns an access token for the specified scopes.
	// The token should be cached and refreshed as needed.
	GetToken(ctx context.Context, options TokenRequestOptions) (AccessToken, error)
}

// TokenRequestOptions configures token acquisition.
type TokenRequestOptions struct {
	// Scopes specifies the required OAuth2 scopes for the token.
	// For Azure AI Foundry, use []string{DefaultScope}.
	Scopes []string
}

// AccessToken represents an Azure access token with expiration.
type AccessToken struct {
	// Token is the bearer token string to use in Authorization header.
	Token string

	// ExpiresOn indicates when the token expires.
	// Implementations should refresh tokens before expiration.
	ExpiresOn time.Time
}

// tokenCache wraps a TokenCredential with caching to avoid unnecessary token refreshes.
// It stores the last acquired token and only refreshes when expired or near expiration.
type tokenCache struct {
	credential TokenCredential
	token      *AccessToken
	buffer     time.Duration // refresh this much before expiry
}

// newTokenCache creates a token cache with a 5-minute buffer before expiration.
func newTokenCache(credential TokenCredential) *tokenCache {
	return &tokenCache{
		credential: credential,
		buffer:     5 * time.Minute,
	}
}

// getToken returns a cached token or acquires a new one if expired.
func (c *tokenCache) getToken(ctx context.Context) (string, error) {
	now := time.Now()

	// Return cached token if still valid (with buffer)
	if c.token != nil && c.token.ExpiresOn.Add(-c.buffer).After(now) {
		return c.token.Token, nil
	}

	// Acquire new token
	token, err := c.credential.GetToken(ctx, TokenRequestOptions{
		Scopes: []string{DefaultScope},
	})
	if err != nil {
		return "", err
	}

	c.token = &token
	return token.Token, nil
}
