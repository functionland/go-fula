package drive

import "strings"

// PathSlice convert a path into a slice containing segments of the path
// TODO: make the unit test for PathSlice
func PathSlice(path string) []string {
	sp := strings.Split(path, "/")

	for idx, s := range sp {
		if s == "" {
			sp = append(sp[:idx], sp[idx+1:]...)
		}
	}

	return sp
}
