package naming

import "strings"

func ModuleName(name string) string {
	return strings.NewReplacer("/", "_", "-", "_").Replace(name)
}
