package analyzer

import (
	"sort"
	"strings"
)

func optimizeFields(fields []fieldInfo) []fieldInfo {
	var hot []fieldInfo
	var cold []fieldInfo

	for _, f := range fields {
		if isHotField(f.name) {
			hot = append(hot, f)
		} else {
			cold = append(cold, f)
		}
	}

	sortByAlign(hot)
	sortByAlign(cold)

	return append(hot, cold...)
}

func sortByAlign(fields []fieldInfo) {
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].align > fields[j].align
	})
}

func isHotField(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "count") ||
		strings.Contains(name, "flag") ||
		strings.Contains(name, "state") ||
		strings.Contains(name, "status")
}
