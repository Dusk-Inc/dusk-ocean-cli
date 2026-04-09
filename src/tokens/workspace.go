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
)
