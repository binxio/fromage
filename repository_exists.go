package main

import (
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"log"
)

var cache = map[string]bool{}

func RepositoryExists(reference name.Reference, verbose bool) bool {
	name := reference.Context().String()
	if result, ok := cache[name]; ok {
		return result
	}
	_, err := crane.Head(reference.Name())
	// _, err := crane.ListTags(reference.Context().String())
	if err != nil && verbose {
		log.Printf("DEBUG: no manifest found for %s, %s", name, err)
	}

	cache[name] = err == nil
	return err == nil
}
