package tag

import (
	"github.com/google/go-containerregistry/pkg/name"
	"testing"
)

func TestMakeTag(t *testing.T) {
	var outputs = []Tag{
		{Literal: "1.15", Version: []int{1, 15}},
		{Literal: "1.15.2", Version: []int{1, 15, 2}},
		{Literal: "v1.15.2", Version: []int{1, 15, 2}, Prefix: "v", Category: "v|"},
		{Literal: "1 .15", Version: []int{1}, Suffix: " .15", Category: "| .15"},
		{Literal: "7.2-fpm", Version: []int{7, 2}, Suffix: "-fpm", Category: "|-fpm"},
		{Literal: "7.2-fpm", Version: []int{7, 2}, Suffix: "-fpm", Category: "|-fpm"},
		{Literal: "latest", Version: []int{}, Prefix: "latest", Category: "latest|"},
		{Literal: "27efa74b9c", Version: []int{}, Prefix: "27efa74b9c", Category: "27efa74b9c|"},
		{Literal: "v0.5-f1c5941", Version: []int{0, 5}, Prefix: "v", Suffix: "-f1c5941", Category: "v|<git-commit-sha>"},
		{Literal: "0.6.1-1-gf1c5941", Version: []int{0, 6, 1}, Suffix: "-1-gf1c5941", Category: "|<git-describe>"},
	}
	for i, expect := range outputs {
		tag := MakeTag(expect.Literal)
		if !tag.Equals(outputs[i]) {
			t.Fatalf("expected tag to be %v, got %v", outputs[i], tag)
		}
		if tag.String() != expect.Literal {
			t.Fatalf("expected tag.String() to be %s, got %s", expect.Literal, tag.String())
		}
	}
}

type listAllTagsData struct {
	input               string
	output              []Tag
	greaterCountMinimum int
}

func contains(a []Tag, x Tag) bool {
	for _, n := range a {
		if n.Equals(x) {
			return true
		}
	}
	return false
}

func TestListAllTags(t *testing.T) {
	var tests = []listAllTagsData{{
		"php:7.2-fpm",
		[]Tag{MakeTag("7.2-fpm"), MakeTag("7.3-fpm"), MakeTag("7.4-fpm")},
		2,
	},
		{
			"golang:1.12",
			[]Tag{MakeTag("1.12"), MakeTag("1.13"), MakeTag("1.14"), MakeTag("1.15")},
			3,
		},
	}

	for _, test := range tests {
		reference, _ := name.ParseReference(test.input)
		result, err := ListAllTags(reference.Context().Name())
		if err != nil {
			t.Fatal(err)
		}

		categories := MakeTagCategories(result)

		taggedReference, _ := reference.(name.Tag)
		tag := MakeTag(taggedReference.TagStr())

		if tags, ok := categories[tag.Category]; ok {
			for _, o := range test.output {
				if !contains(tags, o) {
					t.Fatalf("expected tag %s in %v", o.String(), test.output)
				}
			}

			greaterTags := tags.FindGreaterThan(tag)
			if len(greaterTags) < test.greaterCountMinimum {
				t.Fatalf("expected at least %d tags greater than %s, found %v",
					test.greaterCountMinimum, test.input, greaterTags)
			}
		}
	}
}

type getNextVersionTest struct {
	input  string
	error  bool
	limit  *Level
	output string
}

func TestGetNextVersion(t *testing.T) {
	var minor Level = MINOR
	var major Level = MAJOR
	var patch Level = PATCH
	var tests = []getNextVersionTest{
		{"php:7.99-fpm", false, nil,"php:8.0-fpm"},
		{"php:7.2-fpm", false, &major,"php:7.3-fpm"},
		{"php:7.2-fpm", false, &minor,"php:7.2-fpm"},
		{"php:7.2.0-fpm", false, nil,"php:7.2.1-fpm"},
		{"php:7.2.0-fpm", false, &patch,"php:7.2.0-fpm"},
		{"php:7.2-fpm", false, &patch,"php:7.2-fpm"},
		{"php:7.2-fpm", false, nil,"php:7.3-fpm"},
		{"php:7.3-fpm", false, nil, "php:7.4-fpm"},
		{"php:7.3-xfpm", false, nil, "php:7.3-xfpm"},
		{"deadbeef:1.0", true, nil, "deadbeef:1.0"},
		{"golang:1.12.0", false, nil, "golang:1.12.1"},
		{"golang:1.12.99", false, nil, "golang:1.13.0"},
		{"golang:1.12.99", false, &minor, "golang:1.12.99"},
		{"index.docker.io/library/golang:1.12.6", false, nil, "index.docker.io/library/golang:1.12.7"},
		{"golang:latest", false, nil, "golang:latest"},
		{"golang:least", false, nil, "golang:least"},
		{"php:28.1-fpm", false, nil, "php:28.1-fpm"},
	}
	for _, test := range tests {
		i, _ := name.ParseReference(test.input)
		input, _ := i.(name.Tag)
		output, err := GetNextVersion(input, test.limit)
		if err == nil {
			expect, _ := name.ParseReference(test.output)

			if expect.Name() != output.Name() {
				if test.limit == nil {
					t.Fatalf("expected next version of %s to be %s got %s", input, test.output, output)
				} else {
					t.Fatalf("expected next version of %s within %s to be %s got %s", input, *test.limit, test.output, output)
				}
			}

			if output.String() != test.output {
				t.Fatalf("expected next version of %s to be %s got %s", input, test.output, output)
			}
		} else {
			if !test.error {
				t.Fatalf("failed to get next version of %s, %s", input, err)
			}
		}
	}
}

type getNextVersionsTest struct {
	input  []string
	error  bool
	output []string
}

func TestGetNextVersions(t *testing.T) {
	var tests = []getNextVersionsTest{
		{[]string{"php:7.2-fpm", "deadbeef:1.0", "golang:1.12.0"}, true,
			[]string{"php:7.3-fpm", "deadbeef:1.0", "golang:1.12.1"},
		},
		{[]string{"php@sha256:87c8a1d8f54f3aa4e05569e8919397b65056aa71cdf48b7f061432c98475eee9", "golang:1.12.0"}, false,
			[]string{"golang:1.12.1"},
		},
		{[]string{"php:7.2-fpm", "deadbeef:1.0", "golang:1.12.0"}, true,
			[]string{"php:7.3-fpm", "deadbeef:1.0", "golang:1.12.1"},
		},

	}
	for _, test := range tests {
		var i = make([]name.Reference, 0, len(test.input))
		for _, r := range test.input {
			ref, err := name.ParseReference(r)
			if err != nil {
				t.Fatal(err)
			}
			i = append(i, ref)
		}
		var o = make([]name.Reference, 0, len(test.input))
		for _, r := range test.output {
			ref, err := name.ParseReference(r)
			if err != nil {
				t.Fatal(err)
			}
			o = append(o, ref)
		}
		output, err := GetNextVersions(i, nil)
		if test.error != (err != nil) {
			t.Fatalf("expected error to be %v was %v", test.error, (err != nil))
		}

		for idx, expect := range o {
			if idx >= len(output) {
				t.Fatalf("missing expected output %s", o)
			}

			if expect.Name() != output[idx].Name() {
				t.Fatalf("expected %s, got %s", expect.Name(), output[idx].Name())
			}
		}
	}
}
