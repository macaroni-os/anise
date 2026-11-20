/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/

package helpers

import (
	"io/ioutil"

	fileHelper "github.com/macaroni-os/anise/pkg/helpers/file"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
)

// ChartFileS is an helper that takes a string and construct a chart.File slice from it
func ChartFileS(s string) []*chart.File {
	return []*chart.File{
		{Name: "templates", Data: []byte(s)},
	}
}

// ChartFile reads all the given files and returns a slice of []*chart.File
// containing the raw content and the file name for each file
func ChartFile(s ...string) []*chart.File {
	files := []*chart.File{}
	for _, c := range s {
		raw, err := ioutil.ReadFile(c)
		if err != nil {
			return files
		}
		files = append(files, &chart.File{Name: c, Data: raw})
	}

	return files
}

// RenderHelm renders the template string with helm
func RenderHelm(files []*chart.File, values, d map[string]interface{}) (string, error) {

	// We slurp all the files into one here. This is not elegant, but still works.
	// As a reminder, the files passed here have on the head the templates in the 'templates/' folder
	// of each anise tree, and it have at the bottom the package buildpsec to be templated.
	// TODO: Replace by correctly populating the files so that the helm render engine templates it
	// correctly
	toTemplate := ""
	for _, f := range files {
		toTemplate += string(f.Data)
	}

	c := &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    "",
			Version: "",
		},
		Templates: ChartFileS(toTemplate),
		Values:    map[string]interface{}{"Values": values},
	}

	v, err := chartutil.CoalesceValues(c, map[string]interface{}{"Values": d})
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}
	out, err := engine.Render(c, v)
	if err != nil {
		return "", errors.Wrap(err, "while rendering template")
	}

	return out["templates"], nil
}

type templatedata map[string]interface{}

// UnMarshalValues unmarshal values files and joins them into a unique templatedata
// the join happens from right to left, so any rightmost value file overwrites the content of the ones before it.
func UnMarshalValues(values []string) (templatedata, error) {
	dst := templatedata{}
	if len(values) > 0 {
		for _, bv := range reverse(values) {
			current := templatedata{}

			defBuild, err := ioutil.ReadFile(bv)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+bv)
			}
			err = yaml.Unmarshal(defBuild, &current)
			if err != nil {
				return nil, errors.Wrap(err, "rendering file "+bv)
			}
			if err := mergo.Merge(&dst, current); err != nil {
				return nil, errors.Wrap(err, "merging values file "+bv)
			}
		}
	}
	return dst, nil
}

func reverse(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func RenderFiles(files []*chart.File, valuesFile string, defaultFile ...string) (string, error) {
	if !fileHelper.Exists(valuesFile) {
		return "", errors.New("file does not exist: " + valuesFile)
	}
	val, err := ioutil.ReadFile(valuesFile)
	if err != nil {
		return "", errors.Wrap(err, "reading file: "+valuesFile)
	}

	var values templatedata
	if err = yaml.Unmarshal(val, &values); err != nil {
		return "", errors.Wrap(err, "unmarshalling values")
	}

	dst, err := UnMarshalValues(defaultFile)
	if err != nil {
		return "", errors.Wrap(err, "unmarshalling values")
	}

	return RenderHelm(files, values, dst)
}
