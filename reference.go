package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
	"text/tabwriter"
)

type DockerfileFromReference struct {
	Reference string   `json:"image,omitempty" yaml:"image"`
	Path      string   `json:"path,omitempty"`
	Branch    string   `json:"branch,omitempty"`
	Newer     []string `json:"newer,omitempty"`
}
type DockerfileFromReferences []*DockerfileFromReference

func (r DockerfileFromReferences) ExtractReferences() []string {
	refs := make(map[string]bool, len(r))
	for _, reference := range r {
		refs[reference.Reference] = true
	}
	names := make([]string, 0, len(refs))
	for name, _ := range refs {
		names = append(names, name)
	}
	return names
}

func (r DockerfileFromReferences) OutputOnlyReferences(format string, noHeader bool) {
	result := r.ExtractReferences()

	if format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		_ = encoder.Encode(r)
	} else if format == "yaml" {
		encoder := yaml.NewEncoder(os.Stdout)
		_ = encoder.Encode(r)
	} else {
		w := tabwriter.NewWriter(os.Stdout, 1, 8, 0, '\t', tabwriter.TabIndent)
		if !noHeader {
			fmt.Fprintf(w, "%s\n", "REFERENCE")
		}
		for _, reference := range result {
			fmt.Fprintf(w, "%s\n", reference)
		}
		w.Flush()
	}
}

func (r DockerfileFromReferences) Output(format string, noHeader bool) {

	if format == "json" {
		encoder := json.NewEncoder(os.Stdout)
		_ = encoder.Encode(r)
	} else if format == "yaml" {
		encoder := yaml.NewEncoder(os.Stdout)
		_ = encoder.Encode(r)
	} else {
		w := tabwriter.NewWriter(os.Stdout, 1, 8, 1, '\t', tabwriter.TabIndent)
		if !noHeader {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "IMAGE", "PATH", "BRANCH", "NEWER")
		}
		for _, reference := range r {
			var newer = "-"
			if reference.Newer != nil {
				newer = strings.Join(reference.Newer, ",")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", reference.Reference, reference.Path, reference.Branch, newer)
		}
		w.Flush()
	}
}
