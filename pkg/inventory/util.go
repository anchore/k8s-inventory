package inventory

import "regexp"

func checkForRegexMatch(regex string, value string) bool {
	return regexp.MustCompile(regex).MatchString(value)
}

func processAnnotationsOrLabels(annotationsOrLabels map[string]string, include []string) map[string]string {
	if len(include) == 0 {
		return annotationsOrLabels
	}
	toReturn := make(map[string]string)
	for key, val := range annotationsOrLabels {
		for _, includeKey := range include {
			if checkForRegexMatch(includeKey, key) {
				toReturn[key] = val
			}
		}
	}
	return toReturn
}
