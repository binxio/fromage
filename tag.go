package main

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"regexp"
	"sort"
	"strconv"
)

type Tag struct {
	Literal  string
	Prefix   string
	Suffix   string
	Version  []int
	Category string
}

type Tags []Tag

var (
	semVerRegExp      = regexp.MustCompile(`(?m)^(?P<prefix>[^0-9]*)(?P<major>[0-9]+)(\.(?P<minor>[0-9]+))?(\.(?P<patch>[0-9]+))?(?P<suffix>\W.*)?$`)
	semVerRegExpNames = semVerRegExp.SubexpNames()
)

func MakeTag(tag string) Tag {
	var result = Tag{
		Literal: tag,
		Version: make([]int, 0, 3),
	}

	matches := semVerRegExp.FindStringSubmatch(tag)
	if matches != nil {
		for i, name := range semVerRegExpNames {
			switch name {
			case "prefix":
				result.Prefix = matches[i]
			case "suffix":
				result.Suffix = matches[i]
			case "major", "minor", "patch":
				if len(matches[i]) > 0 {
					level, _ := strconv.Atoi(matches[i])
					result.Version = append(result.Version, level)
				}
			default:
				// ignore
			}
		}
	} else {
		result.Prefix = tag
	}
	if result.Prefix != "" || result.Suffix != "" {
		result.Category = fmt.Sprintf("%s|%s", result.Prefix, result.Suffix)
	}

	return result
}

func compareInt(a int, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func compareVersion(a []int, b []int) int {
	var result int
	for i, v := range a {
		if i < len(b) {
			result = compareInt(v, b[i])
			if result != 0 {
				return result
			}
		} else {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	return 0
}

func (a Tag) Compare(b Tag) int {
	return compareVersion(a.Version, b.Version)
}

func (a Tag) Equals(b Tag) bool {
	return a.Literal == b.Literal &&
		a.Prefix == b.Prefix &&
		a.Suffix == b.Suffix &&
		a.Category == b.Category &&
		compareVersion(a.Version, b.Version) == 0
}

func ListAllTags(reference name.Reference) ([]Tag, error) {

	tags, err := crane.ListTags(reference.Context().String())
	if err != nil {
		return nil, fmt.Errorf("could not retrieve tags for %s", reference)
	}

	result := make([]Tag, len(tags), len(tags))
	for i, tag := range tags {
		result[i] = MakeTag(tag)
	}

	return result, nil
}

type TagList []Tag
type TagCategories map[string]TagList

func (l TagList) FindGreaterThan(tag Tag) TagList {
	result := make(TagList, 0)
	for _, t := range l {
		if len(t.Version) == len(tag.Version) && tag.Compare(t) < 0 {
			result = append(result, t)
		}
	}
	return result
}

func (a TagList) Len() int           { return len(a) }
func (a TagList) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a TagList) Less(i, j int) bool { return (a[i]).Compare(a[j]) < 0 }

func MakeTagCategories(tags TagList) TagCategories {
	var result = make(map[string]TagList)
	for _, tag := range tags {
		if list, ok := result[tag.Category]; ok {
			result[tag.Category] = append(list, tag)
		} else {
			result[tag.Category] = []Tag{tag}
		}
	}
	for _, tags := range result {
		sort.Sort(tags)
	}
	return result
}
