package releasetool

// Copied with love from helm's pkg/storage/storage.go

import "fmt"

// makeKey concatenates a release name and version into
// a string with format ```<release_name>#v<version>```.
// This key is used to uniquely identify storage objects.
func makeKey(rlsname string, version int32) string {
	return fmt.Sprintf("%s.v%d", rlsname, version)
}
