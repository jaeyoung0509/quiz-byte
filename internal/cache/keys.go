package cache

import "strings"

const (
	GlobalKeyPrefix = "quizbyte"
)

// GenerateCacheKey generates a cache key for a given service, object type, and identifier.
// If paramsKey are provided, they are joined by "_" and appended to the cache key.
func GenerateCacheKey(serviceName, objectType, identifier string, paramsKey ...string) string {
	baseKey := strings.Join([]string{GlobalKeyPrefix, serviceName, objectType, identifier}, ":")
	if len(paramsKey) > 0 {
		return strings.Join([]string{baseKey, strings.Join(paramsKey, "_")}, ":")
	}
	return baseKey
}
