package install

import (
	"testing"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
)

func TestValidateInstallFlow(t *testing.T) {
	t.Run("domain__allowed_paths__return_nil", func(t *testing.T) {
		cases := []struct {
			target installTarget
			dep    installDependency
		}{
			{
				target: installTarget{Kind: targetGlobalLib, Name: "g"},
				dep:    installDependency{kind: dependencyGlobalLib, name: "g2"},
			},
			{
				target: installTarget{Kind: targetProject, Name: "p"},
				dep:    installDependency{kind: dependencyGlobalLib, name: "g"},
			},
			{
				target: installTarget{Kind: targetAppLib, App: "app", Name: "lib"},
				dep:    installDependency{kind: dependencyAppLib, app: "app", name: "lib2"},
			},
			{
				target: installTarget{Kind: targetAppLib, App: "app", Name: "lib"},
				dep:    installDependency{kind: dependencyGlobalLib, name: "g"},
			},
			{
				target: installTarget{Kind: targetAppLib, App: "app", Name: "lib"},
				dep:    installDependency{kind: dependencyProject, name: "p"},
			},
			{
				target: installTarget{Kind: targetService, App: "app", Name: "svc"},
				dep:    installDependency{kind: dependencyAppLib, app: "app", name: "lib"},
			},
			{
				target: installTarget{Kind: targetService, App: "app", Name: "svc"},
				dep:    installDependency{kind: dependencyGlobalLib, name: "g"},
			},
			{
				target: installTarget{Kind: targetService, App: "app", Name: "svc"},
				dep:    installDependency{kind: dependencyProject, name: "p"},
			},
		}
		for _, entry := range cases {
			if err := validateInstallFlow(entry.target, entry.dep); err != nil {
				t.Fatalf("expected allowed dependency, got %v", err)
			}
		}
	})

	t.Run("boundary__app_lib_cross_app__returns_error", func(t *testing.T) {
		target := installTarget{Kind: targetAppLib, App: "alpha", Name: "lib"}
		dep := installDependency{kind: dependencyAppLib, app: "beta", name: "lib"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("boundary__service_cross_app_lib__returns_error", func(t *testing.T) {
		target := installTarget{Kind: targetService, App: "alpha", Name: "svc"}
		dep := installDependency{kind: dependencyAppLib, app: "beta", name: "lib"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__invalid_dependency_kind__returns_error", func(t *testing.T) {
		target := installTarget{Kind: targetGlobalLib, Name: "lib"}
		dep := installDependency{kind: dependencyProject, name: "proj"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("complement__unsupported_target__returns_error", func(t *testing.T) {
		target := installTarget{Kind: targetKind("unknown")}
		dep := installDependency{kind: dependencyGlobalLib, name: "lib"}
		if err := validateInstallFlow(target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestEnsureNoCycles(t *testing.T) {
	t.Run("domain__non_cyclic_dependency__returns_nil", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("a"),
				workspace.MakeLibrary("b"),
			},
			nil,
			nil,
		)
		target := installTarget{Kind: targetGlobalLib, Name: "a"}
		dep := installDependency{kind: dependencyGlobalLib, name: "b"}
		if err := ensureNoCycles(config, target, dep); err != nil {
			t.Fatalf("expected no cycle, got %v", err)
		}
	})

	t.Run("boundary__cycle_detected__returns_error", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("a", "b"),
				workspace.MakeLibrary("b"),
			},
			nil,
			nil,
		)
		target := installTarget{Kind: targetGlobalLib, Name: "b"}
		dep := installDependency{kind: dependencyGlobalLib, name: "a"}
		if err := ensureNoCycles(config, target, dep); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("boundary__service_target__skips_cycle_check", func(t *testing.T) {
		config := workspace.MakeConfig(
			[]workspace.WorkspaceLibrary{
				workspace.MakeLibrary("a", "b"),
				workspace.MakeLibrary("b"),
			},
			nil,
			nil,
		)
		target := installTarget{Kind: targetService, App: "app", Name: "svc"}
		dep := installDependency{kind: dependencyGlobalLib, name: "a"}
		if err := ensureNoCycles(config, target, dep); err != nil {
			t.Fatalf("expected no cycle check for service, got %v", err)
		}
	})
}
