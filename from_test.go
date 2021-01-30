package main

import (
	"github.com/binxio/fromage/tag"
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
FROM golang@sha256:d0e79a9c39cdb3d71cc45fec929d1308d50420b79201467ec602b1b80cc314a8 as builder #comment

FROM builder als runtime
`),
			"golang@sha256:d0e79a9c39cdb3d71cc45fec929d1308d50420b79201467ec602b1b80cc314a8",
			false,
			[]byte(`
FROM golang@sha256:d0e79a9c39cdb3d71cc45fec929d1308d50420b79201467ec602b1b80cc314a8 as builder #comment

FROM builder als runtime
`),
		},
		{
			[]byte(`
FROM golang:1.12 as builder #comment

FROM builder as runtime
`),
			"golang:1.12",
			true,
			[]byte(`
FROM golang:1.13 as builder #comment

FROM builder as runtime
`),
		},
	}

	for _, test := range tests {
		r, err := name.ParseReference(test.reference)
		if err != nil {
			t.Fatal(err)
		}
		reference, _ := r.(name.Tag)
		nextRef, _ := tag.GetNextVersion(reference, nil, false)
		result, updated := UpdateFromStatements(test.dockerfile, reference, nextRef, "./Dockerfile", true)
		if updated != test.updated {
			t.Fatalf("expected updated to be %v, in %s", test.updated, string(test.dockerfile))
		}
		if string(result) != string(test.newDockerfile) {
			t.Fatalf("update did not match expected result")
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
FROM golang:1.12.17 as builder #comment

FROM builder as runtime

FROM php:7.2-fpm
`),
			true,
			[]byte(`
FROM golang:1.13.0 as builder #comment

FROM builder as runtime

FROM php:7.3-fpm
`),
		},
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
FROM golang:1.12.0 as builder #comment

FROM builder as runtime

FROM php:7.2-fpm
`),
			true,
			[]byte(`
FROM golang:1.12.1 as builder #comment

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
		newDockerfile, updated := UpdateAllFromStatements(test.dockerfile, "./Dockerfile", nil, false, true)

		if test.updated != updated {
			t.Fatalf("expected updated to be %v in %s", test.updated, string(test.dockerfile))
		}

		if string(test.newDockerfile) != string(newDockerfile) {
			t.Fatalf("expected new Dockerfile to be\n%s\ngot:\n%s\n",
				string(test.newDockerfile), string(newDockerfile))
		}
	}
}
