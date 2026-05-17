package domains

import "fmt"

// APIError wraps an upstream provider error in a stable shape.
type APIError struct {
	Provider   string
	StatusCode int
	Code       string
	Type       string
	Message    string
	RequestID  string
	Cause      error
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Provider, e.Message, e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
