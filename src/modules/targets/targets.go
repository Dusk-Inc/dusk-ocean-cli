package targets

import (
	"fmt"

	"github.com/dusk-inc/dusk-ocean/repos/projects/dusk-ocean/src/modules/workspace"
)

func FormatTargetLabel(target workspace.Target) string {
	switch target.Kind {
	case workspace.TargetService:
		return fmt.Sprintf("service %s/%s", target.App, target.Name)
	case workspace.TargetAppLib:
		return fmt.Sprintf("app library %s/%s", target.App, target.Name)
	case workspace.TargetGlobalLib:
		return fmt.Sprintf("global library %s", target.Name)
	case workspace.TargetProject:
		return fmt.Sprintf("project %s", target.Name)
	case workspace.TargetTest:
		return fmt.Sprintf("test %s/%s", target.App, target.Name)
	default:
		return target.Name
	}
}
