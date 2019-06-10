/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	gogit "gopkg.in/src-d/go-git.v4"
)

// updateGomodWithTaggedDependencies gets the dependencies at the given tag and fills go.mod.
// If anything is changed, it commits the changes. Returns true if go.mod changed.
func updateGomodWithTaggedDependencies(tag string, depsRepo []string) (bool, error) {
	_, err := os.Stat("go.mod")
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	found := map[string]bool{}
	changed := false

	for _, dep := range depsRepo {
		depPath := filepath.Join("..", dep)
		dr, err := gogit.PlainOpen(depPath)
		if err != nil {
			return false, fmt.Errorf("failed to open dependency repo at %q: %v", depPath, err)
		}

		depPkg, err := fullPackageName(depPath)
		if err != nil {
			return changed, fmt.Errorf("failed to get package at %s: %v", depPath, err)
		}

		commit, err := localOrPublishedTaggedCommitHash(dr, tag)
		if err != nil {
			return false, fmt.Errorf("failed to get tag %s for %q: %v", tag, depPkg, err)
		}

		requireCommand := exec.Command("go", "mod", "edit", "-fmt", "-require", fmt.Sprintf("%s@%s", depPkg, commit))
		requireCommand.Env = append(os.Environ(), "GO111MODULE=on")
		if err := requireCommand.Run(); err != nil {
			return changed, fmt.Errorf("Unable to pin %s in the require section of go.mod to %s: %v", depPkg, commit, err)
		}

		replaceCommand := exec.Command("go", "mod", "edit", "-fmt", "-replace", fmt.Sprintf("%s=%s@%s", depPkg, depPkg, commit))
		replaceCommand.Env = append(os.Environ(), "GO111MODULE=on")
		if err := replaceCommand.Run(); err != nil {
			return changed, fmt.Errorf("Unable to pin %s in the replace section of go.mod to %s: %v", depPkg, commit, err)
		}

		tidyCommand := exec.Command("go", "mod", "tidy")
		tidyCommand.Env = append(os.Environ(), "GO111MODULE=on")
		if err := tidyCommand.Run(); err != nil {
			return changed, fmt.Errorf("Unable to run go mod tidy for %s at %s: %v", depPkg, commit, err)
		}

		found[depPkg] = true
		fmt.Printf("Bumping %s in go.mod to %s\n.", depPkg, commit)
		changed = true
	}

	for _, dep := range depsRepo {
		if !found[dep] {
			fmt.Printf("Warning: dependency %s not found in go.mod.\n", dep)
		}
	}
	return changed, nil
}
