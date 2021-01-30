package main

import (
	"bytes"
	"github.com/binxio/fromage/tag"
	"github.com/google/go-containerregistry/pkg/name"
	"log"
	"regexp"
)

var (
	fromRegExp      = regexp.MustCompile(`(?m)^\s*[Ff][Rr][Oo][Mm]\s+(?P<reference>[^\s]+)(\s*[Aa][Ss]\s+(?P<alias>[^\s]+))?.*$`)
	fromRegExpNames = fromRegExp.SubexpNames()
)

func ExtractFromStatements(content []byte) []string {
	result := make([]string, 0)
	aliases := make(map[string]string, 0)
	references := make(map[string]bool, 0)

	matches := fromRegExp.FindAllSubmatch(content, -1)
	if matches != nil {
		for _, match := range matches {
			alias := ""
			reference := ""
			for i, name := range fromRegExpNames {
				switch name {
				case "reference":
					reference = string(match[i])
				case "alias":
					alias = string(match[i])
				default:
					// ignore
				}
			}
			if alias != "" {
				// register the reference as an alias
				aliases[alias] = reference
			}
			if _, ok := aliases[reference]; !ok {
				// the reference is not pointing to an alias
				if _, ok := references[reference]; !ok {
					// the reference was not yet registered
					references[reference] = true
					result = append(result, reference)
				}
			}
		}
	}
	return result
}

func UpdateFromStatements(content []byte, from name.Reference, to name.Reference, filename string, verbose bool) ([]byte, bool) {
	previous := 0
	updated := false
	result := bytes.Buffer{}
	allMatches := fromRegExp.FindAllSubmatchIndex(content, -1)
	for _, match := range allMatches {
		for i, n := range fromRegExpNames {
			switch n {
			case "reference":
				var err error
				var ref name.Reference
				var start = match[i*2]
				var end = match[i*2+1]
				var s = string(content[start:end])
				if ref, err = name.ParseReference(s); err != nil {
					log.Fatalf("internal error: could not parse %s in %s as container reference, %s",
						s, filename, err)
				}

				if ref.Context().Name() == from.Context().Name() {
					if ref.Identifier() == from.Identifier() {

						updated = true
						if verbose {
							log.Printf("INFO: updating reference %s to %s in %s", ref, to, filename)
						}

						result.Write(content[previous:start])
						result.Write([]byte(to.String()))
						previous = end
					}
				}
			default:
				// ignore
			}
		}
	}
	if previous < len(content) {
		result.Write(content[previous:len(content)])
	}

	return result.Bytes(), updated
}

func UpdateAllFromStatements(content []byte, filename string, pin *tag.Level, latest bool, verbose bool) ([]byte, bool) {
	result := false
	refs := ExtractFromStatements(content)
	var references = make([]name.Reference, 0, len(refs))
	for _, refString := range refs {
		ref, err := name.ParseReference(refString)
		if err != nil {
			log.Fatalf("failed to parse %s into a reference, %v", refString, err)
		}
		references = append(references, ref)
	}

	bumper := MakeBumper(references, pin, latest)
	for _, r := range bumper.bumpOrder {
		from, _ := name.ParseReference(r)
		to, _ := name.ParseReference(bumper.bumpReferences[r])
		if c, updated := UpdateFromStatements(content, from, to, filename, true); updated {
			content = c
			result = true
		}
	}
	return content, result
}
