package provider

import (
	"fmt"
	"sync"

	proxyapi "github.com/free-model/proxy-api-lib"
)

// Registry stores providers by name.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]proxyapi.Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]proxyapi.Provider{}}
}

// Register stores a provider by its Name().
func (r *Registry) Register(provider proxyapi.Provider) error {
	if provider == nil {
		return fmt.Errorf("provider: nil provider")
	}
	name := provider.Name()
	if name == "" {
		return fmt.Errorf("provider: provider name is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
	return nil
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (proxyapi.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}

// Names returns registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
