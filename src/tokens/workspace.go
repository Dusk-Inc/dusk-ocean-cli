package tokens

const (
	DefaultImageTag = "dev"

	TargetAppKind       = "app"
	TargetServiceKind   = "service"
	TargetAppLibKind    = "app-lib"
	TargetGlobalLibKind = "global-lib"
	TargetProjectKind   = "project"
	TargetTestKind      = "test"

	DependencySourceGlobal  = "global"
	DependencySourceProject = "project"

	// Repo kinds accepted by adopt and register.
	RepoKindProject  = "project"
	RepoKindLibrary  = "library"
	RepoKindApp      = "app"
	RepoKindService  = "service"
	RepoKindTemplate = "template"

	// Template kinds accepted by --template-kind. Apps are intentionally
	// excluded — apps are not template-able. The CLI scaffolds the app
	// folder structure directly in code.
	TemplateKindService = "service"
	TemplateKindLibrary = "library"
	TemplateKindProject = "project"

	// RemoteNone is the sentinel string written to a repo entry when no
	// upstream URL is known. Tasks that require a real URL should refuse
	// to run against entries carrying this value.
	RemoteNone = "None"

	// Variable namespace prefixes used by the substitution engine.
	VarNsEnv   = "env"
	VarNsVar   = "var"
	VarNsOcean = "ocean"
	VarNsRepo  = "repo"

	// Default workspace task names pre-populated by `dusk-ocean init` so
	// the slots are visible in the freshly-written ocean.workspace.json
	// and the developer can fill in the command for each one.
	WorkspaceTaskClone            = "clone"
	WorkspaceTaskInit             = "init"
	WorkspaceTaskCreateRemote     = "create_remote"
	WorkspaceTaskCheckoutExisting = "checkout_existing"
	WorkspaceTaskCheckoutNew      = "checkout_new"

	// Standard task names in a repo-level ocean.config.json tasks block.
	RepoConfigTaskInstall   = "install"
	RepoConfigTaskBuild     = "build"
	RepoConfigTaskTest      = "test"
	RepoConfigTaskRun       = "run"
	RepoConfigTaskContain   = "contain"
	RepoConfigTaskAdd       = "add"
	RepoConfigTaskUninstall = "uninstall"
	RepoConfigTaskPrebuild  = "prebuild"
	RepoConfigTaskSetup     = "setup"

	// Workspace directory layout constants.
	RepoDirRoot      = "repos"
	RepoDirApps      = "apps"
	RepoDirLibs      = "libs"
	RepoDirProjects  = "projects"
	RepoDirTemplates = "templates"
	RepoDirServices  = "services"
	RepoDirSandbox   = "sandbox"
)

// DefaultWorkspaceTaskNames is the canonical ordered list of workspace
// task names that `dusk-ocean init` writes into a new ocean.workspace.json
// with empty command strings. The order matches the README's example.
var DefaultWorkspaceTaskNames = []string{
	WorkspaceTaskClone,
	WorkspaceTaskInit,
	WorkspaceTaskCreateRemote,
	WorkspaceTaskCheckoutExisting,
	WorkspaceTaskCheckoutNew,
}
