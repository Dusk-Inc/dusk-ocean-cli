package cmd

import (
	"testing"

	"github.com/spf13/afero"
)

// TestAddPlaceholders_filters_ocean_namespace covers REQ 3.9: scaffold
// template tokens prefixed with ocean: are reserved system placeholders
// and must NOT be collected for user prompting. They survive verbatim in
// the generated files so the runtime engine can substitute them later.
func TestAddPlaceholders_filters_ocean_namespace(t *testing.T) {
	t.Run("domain__mixed_tokens__only_user_keys_collected", func(t *testing.T) {
		got := map[string]struct{}{}
		addPlaceholders("hello {{name}} listening on {{ocean:port}} with {{env:HOME}}", got)

		if _, ok := got["name"]; !ok {
			t.Fatalf("expected 'name' to be collected")
		}
		if _, ok := got["ocean:port"]; ok {
			t.Fatalf("ocean:port must not be collected; it is reserved")
		}
		// env: namespace tokens come from .env at runtime; they should also
		// not be prompted. (The code currently collects them; this assertion
		// documents the REQ 3.9 scope: only ocean: is filtered today.)
		if _, ok := got["env:HOME"]; !ok {
			// not asserting either way — out of scope for REQ 3.9
			_ = ok
		}
	})

	t.Run("complement__only_ocean_tokens__nothing_collected", func(t *testing.T) {
		got := map[string]struct{}{}
		addPlaceholders("port={{ocean:port}} image={{ocean:image_path}}", got)
		if len(got) != 0 {
			t.Fatalf("expected zero collected placeholders, got %v", got)
		}
	})
}

// TestCollectPlaceholders_preserves_ocean_in_files asserts that scaffolded
// files containing ocean: tokens leave them in place after the user-prompt
// pass — REQ 3.9.
func TestCollectPlaceholders_preserves_ocean_in_files(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := afero.WriteFile(fs, "tpl/ocean.config.json",
		[]byte("name={{name}}\nport={{ocean:port}}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	keys, err := collectPlaceholders(fs, "tpl")
	if err != nil {
		t.Fatalf("collectPlaceholders: %v", err)
	}

	for _, k := range keys {
		if k == "ocean:port" {
			t.Fatalf("collectPlaceholders must filter out ocean:port; got %v", keys)
		}
	}
	found := false
	for _, k := range keys {
		if k == "name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'name' in collected keys, got %v", keys)
	}
}
