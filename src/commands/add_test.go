package cmd

import (
	"testing"

	"github.com/spf13/afero"
)

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

		if _, ok := got["env:HOME"]; !ok {

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
