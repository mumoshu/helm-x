package helmx

import (
	"os"
	"path/filepath"
	"strings"
)

type SearchFileOpts struct {
	basePath     string
	matchSubPath string
	fileType     string
}

// SearchFiles returns a slice of files that are within the base path, has a matching sub path and file type
func (r *Runner) SearchFiles(o SearchFileOpts) ([]string, error) {
	var files []string

	err := filepath.Walk(o.basePath, func(path string, info os.FileInfo, err error) error {
		if !strings.Contains(path, o.matchSubPath+"/") {
			return nil
		}
		if !strings.HasSuffix(path, o.fileType) {
			return nil
		}
		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
