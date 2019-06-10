package helmx

import (
	"encoding/json"
	"fmt"
	"github.com/mumoshu/helm-x/pkg/releasetool"
	"io"
	"strings"
)

type AdoptOpts struct {
	*ClientOpts

	Namespace       string
	TillerNamespace string

	Out io.Writer
}

type adoptOption interface {
	SetAdoptOption(*AdoptOpts) error
}

type diffOption interface {
	SetDiffOption(*DiffOpts) error
}

type tillerNamespace struct {
	tillerNs string
}

func (n *tillerNamespace) SetAdoptOption(o *AdoptOpts) error {
	o.TillerNamespace = n.tillerNs
	return nil
}

func (n *tillerNamespace) SetDiffOption(o *DiffOpts) error {
	o.TillerNamespace = n.tillerNs
	return nil
}

func TillerNamespace(tillerNs string) *tillerNamespace {
	return &tillerNamespace{tillerNs: tillerNs}
}

type namespace struct {
	ns string
}

func (n *namespace) SetAdoptOption(o *AdoptOpts) error {
	o.Namespace = n.ns
	return nil
}

func (n *namespace) SetDiffOption(o *DiffOpts) error {
	o.Namespace = n.ns
	return nil
}

func Namespace(ns string) *namespace {
	return &namespace{ns: ns}
}

func (r *Runner) Adopt(release string, resources []string, opts ...adoptOption) error {
	o := &AdoptOpts{}
	for i := range opts {
		if err := opts[i].SetAdoptOption(o); err != nil {
			return err
		}
	}

	tillerNs := o.TillerNamespace
	namespace := o.Namespace

	storage, err := releasetool.NewConfigMapBackedReleaseTool(tillerNs)
	if err != nil {
		return err
	}

	kubectlArgs := []string{"get", "-o=json", "--export"}

	var ns string
	if namespace != "" {
		ns = namespace
	} else {
		ns = "default"
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
