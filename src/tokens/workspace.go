package tokens

const (
	DefaultImageTag = "dev"

	TargetAppKind       = "app"
	TargetServiceKind   = "service"
	TargetAppLibKind    = "app-lib"
	TargetGlobalLibKind = "global-lib"
	TargetProjectKind   = "project"
	TargetTestKind      = "test"
	TargetTemplateKind  = "template"

	DependencySourceGlobal  = "global"
	DependencySourceProject = "project"

	RepoKindProject  = "project"
	RepoKindLibrary  = "library"
	RepoKindApp      = "app"
	RepoKindService  = "service"
	RepoKindTemplate = "template"
	RepoKindInfra    = "infra"
	RepoKindDocs     = "docs"

	TemplateKindService = "service"
	TemplateKindLibrary = "library"
	TemplateKindProject = "project"
	TemplateKindInfra   = "infra"
	TemplateKindDocs    = "docs"

	RemoteNone = "None"

	VarNsEnv   = "env"
	VarNsVar   = "var"
	VarNsOcean = "ocean"
	VarNsRepo  = "repo"

	WorkspaceTaskClone            = "clone"
	WorkspaceTaskInit             = "init"
	WorkspaceTaskCreateRemote     = "create_remote"
	WorkspaceTaskCheckoutExisting = "checkout_existing"
	WorkspaceTaskCheckoutNew      = "checkout_new"

	RepoConfigTaskInstall   = "install"
	RepoConfigTaskBuild     = "build"
	RepoConfigTaskTest      = "test"
	RepoConfigTaskRun       = "run"
	RepoConfigTaskContain   = "contain"
	RepoConfigTaskAdd       = "add"
	RepoConfigTaskUninstall = "uninstall"
	RepoConfigTaskPrebuild  = "prebuild"
	RepoConfigTaskSetup     = "setup"

	RepoDirRoot      = "repos"
	RepoDirApps      = "apps"
	RepoDirLibs      = "libs"
	RepoDirProjects  = "projects"
	RepoDirTemplates = "templates"
	RepoDirServices  = "services"
	RepoDirSandbox   = "sandbox"
	RepoDirInfra     = "infra"
	RepoDirDocs      = "docs"
)

var DefaultWorkspaceTaskNames = []string{
	WorkspaceTaskClone,
	WorkspaceTaskInit,
	WorkspaceTaskCreateRemote,
	WorkspaceTaskCheckoutExisting,
	WorkspaceTaskCheckoutNew,
}
