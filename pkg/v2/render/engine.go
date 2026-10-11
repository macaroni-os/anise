/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package render

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/macaroni-os/anise/pkg/config"
	fhelpers "github.com/macaroni-os/anise/pkg/helpers/file"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// Based on the code available on github.com/Mottainai/lxd-compose project

type RenderEngine struct {
	Config            *config.AniseConfig
	Templates         []*chart.File
	MetadataNamespace string

	Values    map[string]interface{}
	DefValues map[string]interface{}
}

func NewRenderEngine(cfg *config.AniseConfig) *RenderEngine {
	return &RenderEngine{
		Config:            cfg,
		Templates:         []*chart.File{},
		MetadataNamespace: "tpl",
		Values:            make(map[string]interface{}, 0),
		DefValues:         make(map[string]interface{}, 0),
	}
}

func (re *RenderEngine) CloneWithoutValues() *RenderEngine {
	ans := &RenderEngine{
		Config:            re.Config,
		Templates:         re.Templates,
		MetadataNamespace: re.MetadataNamespace,
		Values:            make(map[string]interface{}, 0),
		DefValues:         re.DefValues,
	}

	return ans
}

func (re *RenderEngine) LoadValues(files []string) error {
	for _, f := range files {
		if !fhelpers.Exists(f) {
			continue
		}

		values := make(map[string]interface{}, 0)

		val, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf(
				"error on reading render values file %s: %s",
				f, err.Error())
		}

		if err = yaml.Unmarshal(val, &values); err != nil {
			return fmt.Errorf(
				"error on unmarsh file %s: %s",
				f, err.Error())
		}

		// Merge values
		for k, v := range values {
			re.Values[k] = v
		}
	}

	return nil
}

func (re *RenderEngine) LoadDefaultValues(files []string) error {
	for _, f := range files {
		if !fhelpers.Exists(f) {
			continue
		}

		values := make(map[string]interface{}, 0)

		val, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf(
				"error on reading render default values file %s: %s",
				f, err.Error())
		}

		if err = yaml.Unmarshal(val, &values); err != nil {
			return fmt.Errorf(
				"error on unmarsh file %s: %s",
				f, err.Error())
		}

		// Merge values
		for k, v := range values {
			re.DefValues[k] = v
		}
	}

	return nil
}

func (re *RenderEngine) LoadTemplates(templateDirs []string) error {
	var regexConfs = regexp.MustCompile(`.yaml$`)

	if len(templateDirs) == 0 {
		return nil
	}

	files := []*chart.File{}
	for _, tdir := range templateDirs {
		if !fhelpers.Exists(tdir) {
			continue
		}

		dirEntries, err := os.ReadDir(tdir)
		if err != nil {
			return err
		}

		for _, file := range dirEntries {
			if file.IsDir() {
				continue
			}

			if !regexConfs.MatchString(file.Name()) {
				continue
			}

			content, err := os.ReadFile(path.Join(tdir, file.Name()))
			if err != nil {
				return fmt.Errorf(
					"Error on read template file %s/%s: %s",
					tdir, file.Name(), err.Error())
			}

			files = append(files, &chart.File{
				// Using filename without extension for chart file name
				Name: strings.ReplaceAll(file.Name(), ".yaml", ""),
				Data: content,
			})

		}
	}

	re.Templates = files

	return nil
}

func (re *RenderEngine) RenderFile(file string,
	overrideValues map[string]interface{}) (string, error) {

	if !fhelpers.Exists(file) {
		return "", fmt.Errorf("file %s not found", file)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("error on read file %s", file)
	}

	return re.Render(&raw, overrideValues)
}

func (re *RenderEngine) Render(raw *[]byte,
	overrideValues map[string]interface{}) (string, error) {

	var err error

	values := re.Values
	if len(overrideValues) > 0 {
		for k, v := range overrideValues {
			values[k] = v
		}
	}

	charts := []*chart.File{}

	charts = append(charts, re.Templates...)
	charts = append(charts, &chart.File{
		Name: "templates",
		Data: *raw,
	})

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    re.MetadataNamespace,
			Version: "",
		},
		Templates: charts,
		Values:    map[string]interface{}{"Values": re.DefValues},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{"Values": values})
	if err != nil {
		return "", fmt.Errorf(
			"error on coalesce values %s", err.Error())
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "", fmt.Errorf(
			"Error on rendering: %s", err.Error())
	}

	outTemplate := "templates"
	if re.MetadataNamespace != "" {
		outTemplate = re.MetadataNamespace + "/" + outTemplate
	}

	debugHelmTemplate := os.Getenv("ANISE_HELM_DEBUG")
	if debugHelmTemplate == "1" {
		fmt.Println(out[outTemplate])
	}

	return out[outTemplate], nil
}
