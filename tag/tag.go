package tag

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Level int

const (
	MAJOR Level = 0
	MINOR       = 1
	PATCH       = 2
)

func (l Level) String() string {
	return [...]string{"MAJOR", "MINOR", "PATH"}[l]
}

func MakeLevelFromString(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "MAJOR":
		return MAJOR, nil
	case "MINOR":
		return MINOR, nil
	case "PATCH":
		return PATCH, nil
	default:
		return MAJOR, fmt.Errorf("%s is not a representation of a semantic version level", s)
	}
}

type Tag struct {
	Literal  string
	Prefix   string
	Suffix   string
	Version  []int
	Category string
}

type Tags []Tag
type TagCategories map[string]Tags

var (
	semVerRegExp                 = regexp.MustCompile(`(?m)^(?P<prefix>[^0-9]*)(?P<major>[0-9]+)(\.(?P<minor>[0-9]+))?(\.(?P<patch>[0-9]+))?(?P<suffix>\W.*)?$`)
	semVerRegExpNames            = semVerRegExp.SubexpNames()
	tagCategoryCache             = map[string]TagCategories{}
	gitDescribeSuffixRegExp      = regexp.MustCompile(`(?m)^-((?P<order>[0-9]+)-g)?(?P<sha>[0-9a-f]{6,})(?P<dirty>-dirty)?$`)
	gitDescribeOrderSubExprIndex = findStringIndex(gitDescribeSuffixRegExp.SubexpNames(), "order")
)

func findStringIndex(a []string, item string) int {
	for i, s := range a {
		if s == item {
			return i
		}
	}
	return -1
}

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
		if m := gitDescribeSuffixRegExp.FindStringSubmatch(result.Suffix); m != nil {
			if m[gitDescribeOrderSubExprIndex] == "" {
				result.Category = fmt.Sprintf("%s|%s", result.Prefix, "<git-commit-sha>")
			} else {
				result.Category = fmt.Sprintf("%s|%s", result.Prefix, "<git-describe>")
			}

		} else {
			result.Category = fmt.Sprintf("%s|%s", result.Prefix, result.Suffix)
		}
	}

	return result
}

func (t Tag) IsPatchLevel() bool {
	return t.Version != nil && len(t.Version) == 3
}

func HasSameMajorLevel(t, o Tag) bool {
	return t.Category == o.Category && len(t.Version) >= 1 && len(o.Version) >= 1 &&
		t.Version[0] == o.Version[0]
}

func HasSameMinorLevel(t, o Tag) bool {
	return HasSameMajorLevel(t, o) && len(t.Version) >= 2 && len(o.Version) >= 2 &&
		t.Version[1] == o.Version[1]
}

func HasSamePatchLevel(t, o Tag) bool {
	return HasSameMinorLevel(t, o) && len(t.Version) >= 3 && len(o.Version) >= 3 &&
		t.Version[2] == o.Version[2]
}

func (t Tag) String() string {
	var builder = strings.Builder{}
	builder.WriteString(t.Prefix)
	for i, v := range t.Version {
		if i > 0 {
			builder.WriteRune('.')
		}
		builder.WriteString(fmt.Sprintf("%d", v))
	}
	builder.WriteString(t.Suffix)
	return builder.String()
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

func ListAllTags(reference string) ([]Tag, error) {

	tags, err := crane.ListTags(reference)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve tags for %s, %s", reference, err.Error())
	}

	result := make([]Tag, len(tags), len(tags))
	for i, tag := range tags {
		result[i] = MakeTag(tag)
	}

	return result, nil
}

func (l Tags) FilterByLevel(tag Tag, level Level) (result Tags) {
	m := map[Level]func(Tag, Tag) bool{
		MAJOR: HasSameMajorLevel,
		MINOR: HasSameMinorLevel,
		PATCH: HasSamePatchLevel,
	}

	result = make(Tags, 0, len(l))
	for _, t := range l {
		if m[level](t, tag) {
			result = append(result, t)
		}
	}
	return result
}

func (l Tags) FindGreaterThan(tag Tag) Tags {
	result := make(Tags, 0)
	for _, t := range l {
		if len(t.Version) == len(tag.Version) && tag.Compare(t) < 0 {
			result = append(result, t)
		}
	}
	return result
}

func (l Tags) FindHighestPatchLevel(tag Tag) *Tag {
	if !tag.IsPatchLevel() {
		return nil
	}
	result := l.FilterSameMinorVersion(tag)
	if len(result) == 0 {
		return nil
	}

	return &result[len(result)-1]
}

func (l Tags) FilterSameMinorVersion(tag Tag) Tags {
	return l.FilterByLevel(tag, MINOR)
}

func (a Tags) Len() int           { return len(a) }
func (a Tags) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Tags) Less(i, j int) bool { return (a[i]).Compare(a[j]) < 0 }

func MakeTagCategories(tags Tags) TagCategories {
	var result = make(map[string]Tags)
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

func GetTagsFromCache(reference name.Tag) (Tags, error) {
	tag := MakeTag(reference.TagStr())
	name := reference.Context().String()
	categories, ok := tagCategoryCache[name]
	if !ok {
		tagList, err := ListAllTags(name)
		if err != nil {
			tagList = Tags{}
			return Tags{}, err
		}
		categories = MakeTagCategories(tagList)
		tagCategoryCache[name] = categories
	}

	tagList, ok := categories[tag.Category]
	if ok {
		return tagList, nil
	} else {
		return Tags{}, nil
	}
}

func hasImplicitNamespace(repo string, reg name.Registry) bool {
	return !strings.ContainsRune(repo, '/') && reg.RegistryStr() == name.DefaultRegistry
}

func updateIdentifier(reference name.Tag, identifier string) (next name.Tag) {
	next = reference.Tag(identifier)
	if hasImplicitNamespace(reference.String(), reference.Registry) {
		parts := strings.Split(reference.String(), ":")
		r, err := name.ParseReference(fmt.Sprintf("%s:%s", parts[0], next.TagStr()))
		if err != nil {
			log.Fatalf("ERROR:  internal program error constructing new container reference")
		}
		next, _ = r.(name.Tag)
	}
	return next
}

func GetNextVersion(reference name.Tag, pin *Level) (*name.Tag, error) {
	tagList, err := GetTagsFromCache(reference)
	if err != nil {
		log.Printf("WARNING: %s", err)
		return &reference, err
	}

	tag := MakeTag(reference.TagStr())
	if pin != nil {
		tagList = tagList.FilterByLevel(tag, *pin)
	}


	if successors := tagList.FindGreaterThan(tag); len(successors) > 0 {
		nextTag := updateIdentifier(reference, successors[0].Literal)
		if successors[0].IsPatchLevel() {
			successors = successors.FilterByLevel(successors[0], MINOR)
			nextTag = updateIdentifier(reference, successors[len(successors)-1].Literal)
		}
		return &nextTag, nil
	} else {
		if len(tagList) > 1 {
			log.Printf("INFO: %s is at latest version", reference.String())
		} else {
			if len(tagList) == 1 {
				if tagList[0].Literal != tag.Literal {
					log.Printf("WARNING: only 1 version tag was found for %s: '%s'",
						reference.String(), tagList[0].Literal)
				}
			} else {
				log.Printf("WARNING: no other version tags where found for %s", reference.String())
			}
		}
	}
	return &reference, nil
}

func GetNextVersions(references []name.Reference, within *Level) ([]name.Reference, error) {
	var errors = make([]error, 0)
	var result = make([]name.Reference, 0, len(references))

	for _, r := range references {
		if ref, ok := r.(name.Tag); ok {
			ref, err := GetNextVersion(ref, within)
			if err != nil {
				errors = append(errors, err)
			}
			result = append(result, ref)
		} else {
			log.Printf("WARNING: cannot get next version of %s, as it is not a tagged reference", r.String())
		}
	}
	if len(errors) > 0 {
		return result, fmt.Errorf("%v", errors)
	} else {
		return result, nil
	}
}

func GetAllSuccessorsByString(reference string) ([]Tag, error) {
	if r, err := name.ParseReference(reference); err == nil {
		return GetAllSuccessors(r)
	} else {
		return []Tag{}, err
	}
}

func GetAllSuccessors(reference name.Reference) ([]Tag, error) {
	if r, ok := reference.(name.Tag); ok {
		tagList, err := GetTagsFromCache(r)
		if err != nil {
			return nil, err
		}
		tag := MakeTag(r.TagStr())
		return tagList.FindGreaterThan(tag), nil

	} else {
		return []Tag{}, nil
	}
}
