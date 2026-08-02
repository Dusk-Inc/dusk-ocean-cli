package cmd

import (
	"strings"
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

func TestValidateEntityName(t *testing.T) {
	t.Run("domain__letters_digits_dash_underscore__accepted", func(t *testing.T) {
		for _, name := range []string{"alpha", "Alpha9", "my-app", "my_app", "a1-b_2"} {
			if err := validateEntityName("app", name); err != nil {
				t.Errorf("expected %q to be accepted, got %v", name, err)
			}
		}
	})

	t.Run("boundary__surrounding_whitespace__trimmed_then_accepted", func(t *testing.T) {
		if err := validateEntityName("app", "  alpha  "); err != nil {
			t.Errorf("expected a trimmed name to be accepted, got %v", err)
		}
	})

	t.Run("complement__empty_or_blank__rejected", func(t *testing.T) {
		for _, name := range []string{"", "   ", "\t"} {
			if err := validateEntityName("app", name); err == nil {
				t.Errorf("expected %q to be rejected", name)
			}
		}
	})

	t.Run("complement__inner_whitespace__rejected", func(t *testing.T) {
		if err := validateEntityName("app", "my app"); err == nil {
			t.Fatalf("expected an inner space to be rejected")
		}
	})

	t.Run("chaos__path_and_shell_metacharacters__rejected", func(t *testing.T) {
		for _, name := range []string{"../escape", "app/sub", "app;rm -rf /", "app$(id)", "app\x00", "app.name"} {
			if err := validateEntityName("app", name); err == nil {
				t.Errorf("expected %q to be rejected", name)
			}
		}
	})

	t.Run("domain__kind__names_the_subject_in_the_error", func(t *testing.T) {
		err := validateEntityName("project", "")
		if err == nil {
			t.Fatalf("expected an error")
		}
		if !strings.Contains(err.Error(), "project") {
			t.Fatalf("expected the kind in the message, got %v", err)
		}
	})
}

func TestResolveTemplateChoice(t *testing.T) {
	available := []string{"base-app", "go-app"}

	t.Run("domain__known_flag__resolves", func(t *testing.T) {
		name, resolved, err := resolveTemplateChoice(available, "base-app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !resolved || name != "base-app" {
			t.Fatalf("got (%q, %v), want (base-app, true)", name, resolved)
		}
	})

	t.Run("boundary__empty_flag__defers_to_prompt", func(t *testing.T) {
		name, resolved, err := resolveTemplateChoice(available, "  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved || name != "" {
			t.Fatalf("got (%q, %v), want (\"\", false)", name, resolved)
		}
	})

	t.Run("complement__unknown_flag__errors_listing_available", func(t *testing.T) {
		_, _, err := resolveTemplateChoice(available, "ghost")
		if err == nil {
			t.Fatalf("expected an unknown template to be rejected")
		}
		if !strings.Contains(err.Error(), "base-app") {
			t.Fatalf("expected the available set in the message, got %v", err)
		}
	})

	t.Run("chaos__empty_available_set__rejects_any_flag", func(t *testing.T) {
		if _, _, err := resolveTemplateChoice(nil, "base-app"); err == nil {
			t.Fatalf("expected rejection when nothing is available")
		}
	})
}
