package tokens

const (
	DefaultImageTag = "dev"

	TargetAppKind        = "app"
	TargetServiceKind    = "service"
	TargetAppLibKind     = "app-lib"
	TargetGlobalLibKind  = "global-lib"
	TargetProjectKind    = "project"
	TargetAppProjectKind = "app-project"
	TargetTestKind       = "test"
	TargetTemplateKind   = "template"

	DependencySourceGlobal  = "global"
	DependencySourceProject = "project"

	RepoKindProject  = "project"
	RepoKindLibrary  = "library"
	RepoKindApp      = "app"
	RepoKindService  = "service"
	RepoKindTest     = "test"
	RepoKindTemplate = "template"
	RepoKindInfra    = "infra"
	RepoKindDocs     = "docs"

	TemplateKindService = "service"
	TemplateKindLibrary = "library"
	TemplateKindProject = "project"
	TemplateKindApp     = "app"
	TemplateKindTest    = "test"
	TemplateKindInfra   = "infra"
	TemplateKindDocs    = "docs"

	RemoteNone = "None"

	VarNsEnv   = "env"
	VarNsVar   = "var"
	VarNsOcean = "ocean"
	VarNsRepo  = "repo"

	WorkspaceTaskClone            = "clone"
	WorkspaceTaskInit             = "init"
	WorkspaceTaskInitialCommit    = "initial_commit"
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
	RepoDirJobs      = "jobs"
	RepoDirTesting   = "testing"

	AppSubDirServices      = "services"
	AppSubDirLibs          = "libs"
	AppSubDirProjects      = "projects"
	AppSubDirTesting       = "testing"
	AppSubDirJobsDocker    = "jobs/docker"
	AppSubDirJobsMigration = "jobs/migrations"
	AppSubDirJobsScripts   = "jobs/scripts"
)

var DefaultWorkspaceTaskNames = []string{
	WorkspaceTaskClone,
	WorkspaceTaskInit,
	WorkspaceTaskInitialCommit,
	WorkspaceTaskCreateRemote,
	WorkspaceTaskCheckoutExisting,
	WorkspaceTaskCheckoutNew,
}
