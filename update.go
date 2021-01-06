package main

import (
	"bytes"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"log"
	"regexp"
	"sort"
	"strings"
)

var (
	imageReferencePattern = regexp.MustCompile(`(?P<Name>[a-zA-Z0-9\-\.]+(:[0-9]+)?/[\w\-/]+)((:(?P<Tag>[\w][\w\.-]+))|(@(?P<Digest>[a-zA-Z][a-zA-Z0-9]*:[0-9a-fA-F+\.-_]{32,})))`)
	submatchNames         = imageReferencePattern.SubexpNames()
)

type ContainerImageReference struct {
	Tag    string
	Name   string
	Digest string
}

func NewContainerImageReference(imageReference string) (*ContainerImageReference, error) {
	subMatches := imageReferencePattern.FindStringSubmatch(imageReference)
	if len(subMatches) == 0 {
		return nil, fmt.Errorf("'%s' is not a docker image reference", imageReference)
	}

	var result ContainerImageReference
	for i, name := range submatchNames {
		switch name {
		case "Name":
			result.Name = subMatches[i]
		case "Tag":
			result.Tag = subMatches[i]
		case "Digest":
			result.Digest = subMatches[i]
		default:
			// ignore
		}
	}
	if result.Tag == "" && result.Digest == "" {
		return nil, fmt.Errorf("docker image reference without a Tag or Digest")
	}

	_, err := name.ParseReference(imageReference)
	if err != nil {
		return nil, fmt.Errorf("%s is not a container image reference, %s", imageReference, err)
	}

	return &result, nil
}
func MustNewContainerImageReference(imageReference string) *ContainerImageReference {
	r, err := NewContainerImageReference(imageReference)
	if err != nil {
		log.Fatal(err)
	}
	return r
}

func (r ContainerImageReference) String() string {
	builder := strings.Builder{}
	builder.WriteString(r.Name)
	if r.Tag != "" {
		builder.WriteString(":")
		builder.WriteString(r.Tag)
	}
	if r.Digest != "" {
		builder.WriteString("@")
		builder.WriteString(r.Digest)
	}
	return builder.String()
}

func FindAllContainerImageReference(content []byte) []ContainerImageReference {
	var result = make(ContainerImageReferences, 0)
	allMatches := imageReferencePattern.FindAllIndex(content, -1)
	for _, match := range allMatches {
		s := string(content[match[0]:match[1]])
		r, err := NewContainerImageReference(s)
		if err == nil {
			result = append(result, *r)
		}
	}
	sort.Sort(ContainerImageReferences(result))
	return result.RemoveDuplicates()
}

func (r ContainerImageReference) SameRepository(o ContainerImageReference) bool {
	return r.Name == o.Name
}

func (a ContainerImageReference) Compare(b ContainerImageReference) int {
	return strings.Compare(a.String(), b.String())
}

func (r ContainerImageReference) FindAlternateTags() ([]string, error) {
	result := make([]string, 0)
	latest, err := r.ResolveDigest()
	if err != nil {
		return result, err
	}
	tags, err := crane.ListTags(r.Name)
	if err != nil {
		return result, fmt.Errorf("could not retrieve tags for %s", r.Name)
	}
	for _, tag := range tags {
		tagged, err := ContainerImageReference{Name: r.Name, Tag: tag}.ResolveDigest()
		if err != nil {
			log.Printf("WARNING: could not get digest for image tag %s for %s, %s", tag, r.Name, err)
			err = nil
			continue
		}
		if tagged.Digest == latest.Digest {
			result = append(result, tag)
		}
	}

	return result, nil
}

func UpdateReference(content []byte, reference ContainerImageReference, filename string, verbose bool) ([]byte, bool) {
	previous := 0
	updated := false
	result := bytes.Buffer{}
	allMatches := imageReferencePattern.FindAllIndex(content, -1)
	for _, match := range allMatches {
		s := string(content[match[0]:match[1]])
		r, err := NewContainerImageReference(s)
		if err == nil && r.Name == reference.Name {
			if r.String() != reference.String() {
				updated = true
				if verbose {
					log.Printf("INFO: updating reference %s to %s in %s", r, reference, filename)
				}
				result.Write(content[previous:match[0]])
				result.Write([]byte(reference.String()))
				previous = match[1]
			} else {
				if verbose {
					log.Printf("INFO: %s already up-to-date in %s", r, filename)
				}
			}
		}
	}
	if previous < len(content) {
		result.Write(content[previous:len(content)])
	}

	return result.Bytes(), updated
}

func UpdateReferences(content []byte, references ContainerImageReferences, filename string, verbose bool) ([]byte, bool) {
	updated := false
	changed := false
	for _, ref := range references {
		if content, changed = UpdateReference(content, ref, filename, verbose); changed {
			updated = true
		}
	}
	return content, updated
}

func (r ContainerImageReferences) RemoveDuplicates() []ContainerImageReference {
	keys := make(map[string]bool)
	result := []ContainerImageReference{}
	for _, ref := range r {
		if _, value := keys[ref.String()]; !value {
			keys[ref.String()] = true
			result = append(result, ref)
		}
	}
	return result
}

func (r ContainerImageReference) ResolveDigest() (*ContainerImageReference, error) {
	ref, err := name.ParseReference(r.String())
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get Digest for %s, %s", r, err)
	}

	return &ContainerImageReference{Name: r.Name, Tag: "", Digest: digest.String()}, nil
}

func (r *ContainerImageReference) SetTag(tag string) {
	r.Tag = tag
	r.Digest = ""
}

func (a ContainerImageReferences) ResolveDigest() (ContainerImageReferences, error) {
	result := make([]ContainerImageReference, 0, a.Len())
	for _, r := range a {
		rr, err := r.ResolveDigest()
		if err != nil {
			return nil, err
		}
		log.Printf("resolving repository %s Tag %s to Digest %s\n", r.Name, r.Tag, rr.Digest)
		result = append(result, *rr)
	}
	return result, nil
}

func (a ContainerImageReferences) ResolveTag() (ContainerImageReferences, error) {
	result := make([]ContainerImageReference, 0, a.Len())
	for _, r := range a {
		tags, err := r.FindAlternateTags()
		if err != nil {
			return nil, err
		}
		if len(tags) > 2 {
			log.Printf("WARNING: %s has multiple tags, %v", r, tags)
		}
		for _, tag := range tags {
			if tag != r.Tag {
				log.Printf("resolving repository %s tag '%s' to '%s'\n", r.Name, r.Tag, tag)
				result = append(result, ContainerImageReference{Tag: tag, Name: r.Name})
			}
		}
	}

	return result, nil
}

type ContainerImageReferences []ContainerImageReference

func (a ContainerImageReferences) Len() int           { return len(a) }
func (a ContainerImageReferences) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ContainerImageReferences) Less(i, j int) bool { return (a[i]).Compare(a[j]) < 0 }
