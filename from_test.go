package main

import (
	"github.com/binxio/git-fromage/tag"
	"github.com/google/go-containerregistry/pkg/name"
	"testing"
)

func TestExtractFromStatements(t *testing.T) {
	dockerfile := []byte(`
FROM golang:1.12 as builder #comment

FROM builder als runtime


`)

	result := ExtractFromStatements(dockerfile)
	if len(result) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(result))
	}

	if result[0] != "golang:1.12" {
		t.Fatalf("expected golang:1.12, got %s", result[0])
	}
}

type updateTest struct {
	dockerfile    []byte
	reference     string
	updated       bool
	newDockerfile []byte
}

func TestUpdateFromStatements(t *testing.T) {
	var tests = []updateTest{
		{
			[]byte(`
FROM golang@sha256:f0e10b20de190c7cf4ea7ef410e7229d64facdc5d94514a13aa9b58d36fca647 as builder #comment

FROM builder als runtime
`),
			"golang:1.13",
			true,
			[]byte(`
FROM golang:1.13 as builder #comment

FROM builder als runtime
`),
		},
		{
			[]byte(`
FROM golang:1.12 as builder #comment

FROM builder as runtime
`),
			"golang:1.13",
			true,
			[]byte(`
FROM golang:1.13 as builder #comment

FROM builder as runtime
`),
		},
	}

	for _, test := range tests {
		var reference name.Tag
		if r, err := name.ParseReference(test.reference); err == nil {
			reference, _ = r.(name.Tag)
		}

		result, updated := UpdateFromStatements(test.dockerfile, reference, "./Dockerfile", true)
		if updated != test.updated {
			t.Fatalf("expected updated to be %v, was %v", test.updated, updated)
		}
		if updated && string(result) != string(test.newDockerfile) {
			t.Fatalf("update did nod match expected result")
		}
	}
}

type updateAlltest struct {
	dockerfile    []byte
	updated       bool
	newDockerfile []byte
}

func TestUpdateAllFromStatements(t *testing.T) {
	var tests = []updateAlltest{
		{
			[]byte(`
FROM golang:1.12 as builder #comment

FROM builder as runtime

FROM php:7.2-fpm
`),
			true,
			[]byte(`
FROM golang:1.13 as builder #comment

FROM builder as runtime

FROM php:7.3-fpm
`),
		},
		{
			[]byte(`
FROM golang:latest as builder #comment

FROM builder as runtime

FROM php:latest
`),
			false,
			[]byte(`
FROM golang:latest as builder #comment

FROM builder as runtime

FROM php:latest
`),
		},
	}
	for _, test := range tests {
		var froms = ExtractFromStatements(test.dockerfile)
		var references = make([]name.Reference, 0, len(froms))
		for _, from := range froms {
			ref, _ := name.ParseReference(from)
			references = append(references, ref)
		}
		references, _ = tag.GetNextVersions(references)
		newDockerfile, updated := UpdateAllFromStatements(test.dockerfile,
			references, "./Dockerfile", true)

		if test.updated != updated {
			t.Fatalf("expected updated to be %v", test.updated)
		}

		if string(test.newDockerfile) != string(newDockerfile) {
			t.Fatalf("expected new Dockerfile to be\n%s\ngot:\n%s\n",
				string(test.newDockerfile), string(newDockerfile))
		}
	}
}
