package functions

import (
	"strings"
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/tokens"
)

func TestResolveRepoPath(t *testing.T) {
	cases := []struct {
		name string
		kind string
		repo string
		app  string
		want string
		err  bool
	}{
		{"project", tokens.RepoKindProject, "tooling", "", "repos/projects/tooling", false},
		{"global library", tokens.RepoKindLibrary, "lib-a", "", "repos/libs/lib-a", false},
		{"app-scoped library", tokens.RepoKindLibrary, "lib-a", "app-a", "repos/apps/app-a/libs/lib-a", false},
		{"app", tokens.RepoKindApp, "app-a", "", "repos/apps/app-a", false},
		{"service", tokens.RepoKindService, "svc-a", "app-a", "repos/apps/app-a/services/svc-a", false},
		{"service missing app errors", tokens.RepoKindService, "svc-a", "", "", true},
		{"unknown kind errors", "weird", "x", "", "", true},
		{"missing name errors", tokens.RepoKindProject, "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveRepoPath(tc.kind, tc.repo, tc.app)
			if tc.err {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestValidateRepoKindFlags(t *testing.T) {
	cases := []struct {
		name string
		kind string
		app  string
		err  string
	}{
		{"project no app ok", tokens.RepoKindProject, "", ""},
		{"project with app errors", tokens.RepoKindProject, "app-a", "must not be set"},
		{"app no app ok", tokens.RepoKindApp, "", ""},
		{"app with app errors", tokens.RepoKindApp, "app-a", "must not be set"},
		{"library no app ok", tokens.RepoKindLibrary, "", ""},
		{"library with app ok", tokens.RepoKindLibrary, "app-a", ""},
		{"service with app ok", tokens.RepoKindService, "app-a", ""},
		{"service no app errors", tokens.RepoKindService, "", "is required"},
		{"unknown kind errors", "weird", "", "unknown --kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRepoKindFlags(tc.kind, tc.app)
			if tc.err == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tc.err)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("got %v, want substring %q", err, tc.err)
			}
		})
	}
}
