package domains

import "context"

// Credential supplies authentication material for a request.
type Credential interface {
	AuthorizationHeader(ctx context.Context) (string, error)
}
