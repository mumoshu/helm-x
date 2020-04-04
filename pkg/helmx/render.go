package helmx

import (
	"fmt"
	"github.com/variantdev/chartify"
	"github.com/mumoshu/helm-x/pkg/releasetool"
	"github.com/mumoshu/helm-x/pkg/util"
	"io"
	"strings"
)

type RenderOpts struct {
	*chartify.ChartifyOpts

	IncludeReleaseConfigmap bool
	IncludeReleaseSecret    bool

	Out io.Writer
}

// Render generates K8s manifests for the named release from the chart, and prints the resulting manifests to STDOUT
func (r *Runner) Render(release, chart string, templateOpts RenderOpts) error {
	var additionalFlags string
	additionalFlags += util.CreateFlagChain("set", templateOpts.SetValues)
	additionalFlags += util.CreateFlagChain("f", templateOpts.ValuesFiles)
	if templateOpts.Namespace != "" {
		additionalFlags += util.CreateFlagChain("namespace", []string{templateOpts.Namespace})
	}
	if release != "" {
		additionalFlags += util.CreateFlagChain("name", []string{release})
	}
	if templateOpts.Debug {
		additionalFlags += util.CreateFlagChain("debug", []string{""})
	}
	if templateOpts.ChartVersion != "" {
		additionalFlags += util.CreateFlagChain("--version", []string{templateOpts.ChartVersion})
	}

	command := fmt.Sprintf("%s template %s%s", r.HelmBin(), chart, additionalFlags)
	stdout, stderr, err := r.DeprecatedCaptureBytes(command)
	if err != nil || len(stderr) != 0 {
		return fmt.Errorf(string(stderr))
	}

	var output string

	if templateOpts.IncludeReleaseConfigmap || templateOpts.IncludeReleaseSecret {
		repoNameAndChart := strings.Split(chart, "/")

		chartWithoutRepoName := repoNameAndChart[len(repoNameAndChart)-1]

		ver := templateOpts.ChartVersion

		releaseManifests := []releasetool.ReleaseManifest{}

		if templateOpts.IncludeReleaseConfigmap {
			storage, err := releasetool.NewConfigMapBackedReleaseTool(templateOpts.TillerNamespace)
			if err != nil {
				return err
			}

			releaseManifests = append(releaseManifests, storage.ReleaseToConfigMap)
		}

		if templateOpts.IncludeReleaseSecret {
			storage, err := releasetool.NewSecretBackednReleaseTool(templateOpts.TillerNamespace)
			if err != nil {
				return err
			}

			releaseManifests = append(releaseManifests, storage.ReleaseToSecret)
		}

		output, err = releasetool.TurnHelmTemplateToInstall(chartWithoutRepoName, ver, templateOpts.TillerNamespace, release, templateOpts.Namespace, string(stdout), releaseManifests...)
		if err != nil {
			return err
		}
	} else {
		output = string(stdout)
	}

	fmt.Println(output)

	return nil
}
