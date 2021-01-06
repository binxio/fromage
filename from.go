package main

import (
	"bytes"
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

func UpdateFromStatements(content []byte, reference name.Reference, filename string, verbose bool) ([]byte, bool) {
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

				if ref.Context().Name() == reference.Context().Name() {
					if ref.Identifier() != reference.Identifier() {
						updated = true
						if verbose {
							log.Printf("INFO: updating reference %s to %s in %s", ref, reference, filename)
						}

						result.Write(content[previous:start])
						result.Write([]byte(reference.String()))
						previous = end
					} else {
						if verbose {
							log.Printf("INFO: %s already up-to-date in %s", ref, filename)
						}
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

func UpdateAllFromStatements(content []byte, references []name.Reference, filename string, verbose bool) ([]byte, bool) {
	var result bool

	for _, ref := range references {
		if c, updated := UpdateFromStatements(content, ref, filename, verbose); updated {
			content = c
			result = true
		}
	}

	return content, result
}
