package helmx

import (
	"os"
	"strings"
)

func (r *Runner) IsHelm3() bool {
	if r.isHelm3 {
		return true
	}

	// Support explicit opt-in via environment variable
	if os.Getenv("HELM_X_HELM3") != "" {
		return true
	}

	// Autodetect from `helm verison`
	bytes, err := r.Run(r.HelmBin(), "version", "--client", "--short")
	if err != nil {
		panic(err)
	}

	return strings.HasPrefix(string(bytes), "v3.")
}
