/*
Copyright © 2019-2026 Macaroni OS Linux
See AUTHORS and LICENSE for the license details and contributors.
*/
package tree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	pkg "github.com/macaroni-os/anise/pkg/package"
	spectooling "github.com/macaroni-os/anise/pkg/spectooling"

	"gopkg.in/yaml.v3"
)

func ReadDefinitionFile(defFile string) (*pkg.DefaultPackage, error) {
	data, err := os.ReadFile(defFile)
	if err != nil {
		return nil, errors.New(
			fmt.Sprintf("Error on read file %s: %s",
				defFile, err.Error()))
	}

	ans, err := pkg.NewDefaultPackageFromYaml(data)
	if err != nil {
		return nil, errors.New(
			fmt.Sprintf("Error on parse file %s: %s",
				defFile, err.Error()))
	}

	return ans, nil
}

func ReadDefinitionFromCollection(cFile string, atom *pkg.DefaultPackage) (*pkg.DefaultPackage, error) {

	if filepath.Base(cFile) == "definition.yaml" {
		return nil, fmt.Errorf(
			"trying to read collection file but definition.yaml available for package %s",
			atom.PackageName())
	}

	// Read data an
	data, err := os.ReadFile(cFile)
	if err != nil {
		return nil, fmt.Errorf(
			"Error on read file %s: %s",
			cFile, err.Error())
	}

	coll, err := NewCollection(&data)
	if err != nil {
		return nil, fmt.Errorf(
			"error on parse collection data: %s", err.Error())
	}

	for idx := range coll.Packages {
		if coll.Packages[idx].Matches(atom) {
			return &coll.Packages[idx], nil
		}
	}

	return nil, fmt.Errorf("Package %s not found on collection", atom.HumanReadableString())
}

func ReadDefinitionFileAsMap(defFile string) (*map[string]interface{}, error) {
	data, err := os.ReadFile(defFile)
	if err != nil {
		return nil, fmt.Errorf(
			"Error on read file %s: %s",
			defFile, err.Error())
	}

	ans := make(map[string]interface{}, 0)
	err = yaml.Unmarshal(data, &ans)
	if err != nil {
		return nil, fmt.Errorf(
			"Error on unmarshal data of file %s: %s",
			defFile, err.Error())
	}

	return &ans, nil
}

func WriteDefinitionFile(p pkg.Package, definitionFilePath string) error {
	data, err := spectooling.NewDefaultPackageSanitized(p).Yaml()
	if err != nil {
		return err
	}
	err = os.WriteFile(definitionFilePath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
