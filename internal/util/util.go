package util

import "regexp"

func ObfuscateSensitiveString(value string) string {
	regex := regexp.MustCompile("(token|password|key): (.*?)($|\\n|\\s|\r)")
	return regex.ReplaceAllString(value, "$1: ******$3")
}
