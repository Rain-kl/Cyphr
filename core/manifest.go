package core

import (
	"fmt"
	"strings"
)

// Manifest defines the metadata and dependency declarations for a plugin.
type Manifest struct {
	// Name is the unique identifier for the plugin (e.g. "auth", "user", "order").
	Name string `json:"name" yaml:"name"`

	// Version is the semantic version string of the plugin (e.g. "1.0.0").
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Description gives a brief summary of the plugin capabilities.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Author specifies the author or maintainer of the plugin.
	Author string `json:"author,omitempty" yaml:"author,omitempty"`

	// Dependencies lists the plugin names that this plugin depends on.
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`

	// Metadata holds arbitrary plugin-specific metadata.
	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Validate checks whether the manifest satisfies basic integrity requirements.
func (m Manifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("%w: %w", ErrInvalidManifest, ErrInvalidManifestName)
	}
	return nil
}
