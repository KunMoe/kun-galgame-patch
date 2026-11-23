package config

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Source string

const (
	SourceEnv     Source = "env"
	SourceStore   Source = "store"
	SourceDefault Source = "default"
)

type Var struct {
	Key     string
	Default string
	Value   string
	Source  Source
}

var (
	mu      sync.Mutex
	catalog = map[string]Var{}
	store   storeState
	loaded  time.Time
)

// LoadedAt is when this process last read the store; a stored value written
// after it takes effect only on the next start.
func LoadedAt() time.Time {
	mu.Lock()
	defer mu.Unlock()
	return loaded
}

// Env reads a variable the way Load does. Before Load it sees only the
// environment and the default: the store has not been read yet.
func Env(key, def string) string { return lookup(key, def) }

// lookup reads a variable as environment, then config store, then default,
// and records it so the admin console can show where every value came from.
func lookup(key, def string) string {
	value, source := os.Getenv(key), SourceEnv
	mu.Lock()
	defer mu.Unlock()
	if value == "" {
		if v, ok := store.values[key]; ok && v != "" {
			value, source = v, SourceStore
		}
	}
	if value == "" {
		value, source = def, SourceDefault
	}
	catalog[key] = Var{Key: key, Default: def, Value: value, Source: source}
	return value
}

func Vars() []Var {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Var, 0, len(catalog))
	for _, v := range catalog {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func Fingerprint(value string) string {
	return FingerprintWith(StoreKey(), value)
}

// StoreKey is the parsed KUN_CONFIG_KEK; nil means the store is off.
func StoreKey() []byte {
	mu.Lock()
	defer mu.Unlock()
	return store.key
}

var ownPrefixes = []string{"KUN_", "OAUTH_", "REDIS_", "CORS_"}

// UnreadEnv lists the variables this process was handed that nothing read:
// a compose line whose consumer is gone, or a typo of a real name.
func UnreadEnv() []string {
	mu.Lock()
	defer mu.Unlock()
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if _, read := catalog[name]; read || !HasOwnPrefix(name) {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func HasOwnPrefix(name string) bool {
	for _, p := range ownPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

var secretName = regexp.MustCompile(`SECRET|PASSWORD|TOKEN|KEK|DATABASE_URL|_KEYS?$|_KEY_`)

// IsSecret is decided by name and fails closed: a new variable whose name
// looks like a credential is masked everywhere without anyone listing it.
func IsSecret(key string) bool { return secretName.MatchString(key) }

const (
	KEKEnv         = "KUN_CONFIG_KEK"
	DatabaseURLEnv = "KUN_DATABASE_URL"
)

// IsBootstrap marks what has to be known before the store can be read: the
// database the store lives in and the key that opens it.
func IsBootstrap(key string) bool { return key == KEKEnv || key == DatabaseURLEnv }

var topologyName = regexp.MustCompile(`_(HOST|PORT|ADDR)$|^KUN_SERVER_MODE$`)

// IsTopology marks how the container listens and finds its neighbours. It
// stays in the compose file, reviewed with the deployment it belongs to.
func IsTopology(key string) bool { return topologyName.MatchString(key) }

func Storable(key string) bool { return HasOwnPrefix(key) && !IsBootstrap(key) && !IsTopology(key) }
