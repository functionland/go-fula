package drive

import "strings"

// Convert a path into a slice containing segments of the path
func PathSlice(path string) []string {
	sp := strings.Split(path, "/")

	for idx, s := range sp {
		if s == "" {
			sp = append(sp[:idx], sp[idx+1:]...)
		}
	}

	return sp
}
