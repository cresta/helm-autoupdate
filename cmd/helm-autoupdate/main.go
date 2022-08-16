package main

import (
	"fmt"
	"github.com/cresta/helm-autoupdate/internal/helm"
	"os"
)

func main() {
	currentDirectory, err := os.Getwd()
	if err != nil {
		fmt.Printf("Unable to get current working directory: %v\n", err)
		os.Exit(1)
	}
	l := helm.CachedLoader{
		IndexLoader: &helm.DirectLoader{},
	}
	x := helm.DirectorySearchForChanges{
		Dir: currentDirectory,
	}
	ac, err := helm.LoadFile(".helm-autoupdate.yaml")
	if err != nil {
		fmt.Printf("Unable to load .helm-autoupdate.yaml: %v\n", err)
		os.Exit(1)
	}
	changeFiles, err := x.FindRequestedChanges(ac.ParsedRegex)
	if err != nil {
		fmt.Printf("Unable to find requested changes: %v\n", err)
		os.Exit(1)
	}
	updatedFiles, err := helm.ApplyUpdatesToFiles(&l, ac, changeFiles)
	if err != nil {
		fmt.Printf("Unable to apply updates to files: %v\n", err)
		os.Exit(1)
	}
	err = helm.WriteChangesToFilesystem(updatedFiles)
	if err != nil {
		fmt.Printf("Unable to write changes to filesystem: %v\n", err)
		os.Exit(1)
	}
}
