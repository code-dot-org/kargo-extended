package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/akuity/kargo/extended/pkg/argoworkflows/pkg/plugins/spec"
	argoplugin "github.com/akuity/kargo/extended/pkg/argoworkflows/workflow/util/plugin"
	stepplugincommon "github.com/akuity/kargo/extended/pkg/stepplugin/common"
)

func loadPluginManifest(pluginDir string) (*spec.Plugin, error) {
	manifest, err := os.ReadFile(filepath.Join(pluginDir, "plugin.yaml"))
	if err != nil {
		return nil, err
	}

	p := &spec.Plugin{}
	err = yaml.UnmarshalStrict(manifest, p)
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(pluginDir, "server.*"))
	if err != nil {
		return nil, err
	}
	if len(files) > 1 {
		return nil, fmt.Errorf("plugin %s has more than one server.* file", p.Name)
	}
	if len(files) == 1 {
		code, err := os.ReadFile(files[0])
		if err != nil {
			return nil, err
		}
		p.Spec.Sidecar.Container.Args = []string{string(code)}
	}
	return p, p.Validate()
}

func addHeader(data []byte, header string) []byte {
	return fmt.Appendf(nil, "%s\n%s", header, string(data))
}

func addCodegenHeader(data []byte) []byte {
	return addHeader(data, "# This is an auto-generated file. DO NOT EDIT")
}

func saveBuiltConfigMap(pluginDir string, plug *spec.Plugin) (string, error) {
	cm, err := argoplugin.ToConfigMap(plug)
	if err != nil {
		return "", err
	}
	return saveConfigMap(pluginDir, cm)
}

func saveConfigMap(pluginDir string, cm *corev1.ConfigMap) (string, error) {
	data, err := yaml.Marshal(cm)
	if err != nil {
		return "", err
	}
	cmPath := filepath.Join(
		pluginDir,
		cm.Name+"-configmap.yaml",
	)
	if err := os.WriteFile(cmPath, addCodegenHeader(data), 0o600); err != nil {
		return "", err
	}
	return cmPath, nil
}

func saveReadme(pluginDir string, plug *spec.Plugin) (string, error) {
	readmePath := filepath.Join(pluginDir, "README.md")
	f, err := os.Create(readmePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	tmpl, err := template.New("readme").Parse(`<!-- This is an auto-generated file. DO NOT EDIT -->
# {{.Name}}

* API version: {{.APIVersion}}
* Image: {{.Spec.Sidecar.Container.Image}}
* Namespace: {{if .Namespace}}{{.Namespace}}{{else}}default{{end}}

Install:

    kubectl apply -f {{.Name}}` + stepplugincommon.ConfigMapFilenameSuffix + `

Uninstall:

    kubectl delete configmap {{.Name}}` +
		stepplugincommon.ConfigMapNameSuffix +
		`{{if .Namespace}} -n {{.Namespace}}{{end}}
`)
	if err != nil {
		return "", err
	}
	if err := tmpl.Execute(f, plug); err != nil {
		return "", err
	}
	return readmePath, nil
}
