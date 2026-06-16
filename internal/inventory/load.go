package inventory

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads, parses, and validates an inventory from path.
//
// On parse errors, the returned error preserves the YAML library's
// line:column context. On schema-validation errors, the returned error
// is a *ValidationError carrying every problem found, so callers can
// surface all issues at once.
func Load(path string) (*Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inventory %s: %w", path, err)
	}
	defer f.Close()
	return LoadReader(f)
}

// LoadReader is Load on an open reader (handy for tests and stdin).
func LoadReader(r io.Reader) (*Inventory, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read inventory: %w", err)
	}

	var inv Inventory
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // unknown YAML keys → error
	if err := dec.Decode(&inv); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("inventory is empty")
		}
		return nil, fmt.Errorf("parse inventory: %w", err)
	}
	// Reject trailing content (multi-doc YAML).
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("inventory must contain exactly one YAML document")
	}

	if err := inv.Validate(); err != nil {
		return nil, err
	}
	return &inv, nil
}

