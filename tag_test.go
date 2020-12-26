package main

import (
	"fmt"
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
	}
	for i, expect := range outputs {
		tag := MakeTag(expect.Literal)
		if !tag.Equals(outputs[i]) {
			t.Fatalf("expected tag to be %v, got %v", outputs[i], tag)
		}
	}
}

func TestListAllTags(t *testing.T) {
	for _, ref := range []string{"php:7.2-fpm", "golang:1.12.0"} {
		reference, _ := name.ParseReference(ref)
		result, err := ListAllTags(reference)
		if err != nil {
			t.Fatal(err)
		}
		categories := MakeTagCategories(result)

		taggedReference, _ := reference.(name.Tag)
		tag := MakeTag(taggedReference.TagStr())

		if tags, ok := categories[tag.Category]; ok {
			for _, tag := range tags.FindGreaterThan(tag) {
				fmt.Printf("%s\n", tag.Literal)
			}
		}
	}
}
