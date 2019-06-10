package helmx

import (
	"fmt"
	"github.com/mumoshu/helm-x/pkg/releasetool"
	"strings"
)

func (r *Runner) Template(chart string, templateOpts TemplateOpts) error {
	var additionalFlags string
	additionalFlags += createFlagChain("set", templateOpts.SetValues)
	additionalFlags += createFlagChain("f", templateOpts.ValuesFiles)
	if templateOpts.Namespace != "" {
		additionalFlags += createFlagChain("namespace", []string{templateOpts.Namespace})
	}
	if templateOpts.ReleaseName != "" {
		additionalFlags += createFlagChain("name", []string{templateOpts.ReleaseName})
	}
	if templateOpts.Debug {
		additionalFlags += createFlagChain("debug", []string{""})
	}
	if templateOpts.ChartVersion != "" {
		additionalFlags += createFlagChain("--version", []string{templateOpts.ChartVersion})
	}

	command := fmt.Sprintf("helm template %s%s", chart, additionalFlags)
	stdout, stderr, err := r.DeprecatedCapture(command)
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

		output, err = releasetool.TurnHelmTemplateToInstall(chartWithoutRepoName, ver, templateOpts.TillerNamespace, templateOpts.ReleaseName, templateOpts.Namespace, string(stdout), releaseManifests...)
		if err != nil {
			return err
		}
	} else {
		output = string(stdout)
	}

	fmt.Println(output)

	return nil
}
