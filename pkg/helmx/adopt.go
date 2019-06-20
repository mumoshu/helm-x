package helmx

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/helm/pkg/tiller/environment"

	"github.com/mumoshu/helm-x/pkg/releasetool"
)

type AdoptOpts struct {
	*ClientOpts

	Namespace       string
	TillerNamespace string

	Out io.Writer
}

type AdoptOption interface {
	SetAdoptOption(*AdoptOpts) error
}

// namespace returns the namespace of tiller
// https://github.com/helm/helm/blob/a93ebe17d69e8bf99bdf4880acb40499653dd033/cmd/tiller/tiller.go#L256-L270
func getTillerNamespace() string {
	if ns := os.Getenv("TILLER_NAMESPACE"); ns != "" {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return environment.DefaultTillerNamespace
}

func getActiveContext(pathOptions *clientcmd.PathOptions) string {
	apiConfig, err := pathOptions.GetStartingConfig()
	if err != nil {
		return ""
	}
	if ctx, ok := apiConfig.Contexts[apiConfig.CurrentContext]; ok && ctx != nil {
		return ctx.Namespace
	}
	return ""
}

func (r *Runner) Adopt(release string, resources []string, pathOptions *clientcmd.PathOptions,opts ...AdoptOption) error {
	o := &AdoptOpts{}
	for i := range opts {
		if err := opts[i].SetAdoptOption(o); err != nil {
			return err
		}
	}

	tillerNs := o.TillerNamespace
	if tillerNs == "" {
		tillerNs = getTillerNamespace()
	}
	namespace := o.Namespace

	storage, err := releasetool.NewConfigMapBackedReleaseTool(tillerNs)
	if err != nil {
		return err
	}

	kubectlArgs := []string{"get", "-o=json"}

	var ns string
	if namespace != "" {
		ns = namespace
	} else {
		ns = getActiveContext(pathOptions)
		if ns == "" {
			ns = "default"
		}
	}
	kubectlArgs = append(kubectlArgs, "-n="+ns)

	kubectlArgs = append(kubectlArgs, resources...)

	jsonData, err := r.Run("kubectl", kubectlArgs...)
	if err != nil {
		return err
	}

	var manifest string

	if len(resources) == 1 {
		item := map[string]interface{}{}

		if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
			return err
		}

		yamlData, err := YamlMarshal(item)
		if err != nil {
			return err
		}

		item = export(item)

		yamlData, err = YamlMarshal(item)
		if err != nil {
			return err
		}

		metadata := item["metadata"].(map[string]interface{})
		escaped := fmt.Sprintf("%s.%s", metadata["name"], strings.ToLower(item["kind"].(string)))
		manifest += manifest + fmt.Sprintf("\n---\n# Source: helm-x-dummy-chart/templates/%s.yaml\n", escaped) + string(yamlData)
	} else {
		type jsonVal struct {
			Items []map[string]interface{} `json:"items"`
		}
		v := jsonVal{}

		if err := json.Unmarshal([]byte(jsonData), &v); err != nil {
			return err
		}

		for _, item := range v.Items {
			yamlData, err := YamlMarshal(item)
			if err != nil {
				return err
			}

			item = export(item)

			yamlData, err = YamlMarshal(item)
			if err != nil {
				return err
			}

			metadata := item["metadata"].(map[string]interface{})
			escaped := fmt.Sprintf("%s.%s", metadata["name"], strings.ToLower(item["kind"].(string)))
			manifest += manifest + fmt.Sprintf("\n---\n# Source: helm-x-dummy-chart/templates/%s.yaml\n", escaped) + string(yamlData)
		}
	}

	if manifest == "" {
		return fmt.Errorf("no resources to be adopted")
	}

	if err := storage.AdoptRelease(release, ns, manifest); err != nil {
		return err
	}

	return nil
}
