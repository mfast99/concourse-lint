package analysis

import "time"

// Settings are the rule options from the config file (spec/README.md,
// "Config file"). The zero value is the default of every option.
type Settings struct {
	// ExtraResourceTypes are types the workers provide (CL004).
	ExtraResourceTypes []string
	// ExtraSecretWords make a key secret-like (CS001), lowercase.
	ExtraSecretWords []string
	// AllowedHosts may use http:// or git:// (CS004), lowercase.
	AllowedHosts []string
	// MinCheckInterval is the shortest fine check_every (CB002); 0 means the
	// default of one minute.
	MinCheckInterval time.Duration
}
