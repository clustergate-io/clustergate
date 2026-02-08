package checks

import (
	"fmt"
	"sync"
)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Checker)
)

// Register adds a Checker to the global registry.
// It panics if a check with the same name is already registered.
func Register(c Checker) {
	registryMu.Lock()
	defer registryMu.Unlock()

	name := c.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("check already registered: %s", name))
	}
	registry[name] = c
}

// Get retrieves a Checker by name from the global registry.
func Get(name string) (Checker, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	c, ok := registry[name]
	return c, ok
}

// List returns the names of all registered checks.
func List() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
