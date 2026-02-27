package functions

import (
	"fmt"

)

func FormatTargetLabel(target Target) string {
	switch target.Kind {
	case TargetService:
		return fmt.Sprintf("service %s/%s", target.App, target.Name)
	case TargetAppLib:
		return fmt.Sprintf("app library %s/%s", target.App, target.Name)
	case TargetGlobalLib:
		return fmt.Sprintf("global library %s", target.Name)
	case TargetProject:
		return fmt.Sprintf("project %s", target.Name)
	case TargetTest:
		return fmt.Sprintf("test %s/%s", target.App, target.Name)
	default:
		return target.Name
	}
}
