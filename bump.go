package main

import (
	"github.com/binxio/fromage/tag"
	"github.com/google/go-containerregistry/pkg/name"
)

type Bumper struct {
	bumpReferences map[string]string
	bumpOrder      []string
	dryRun         bool
}

func (b *Bumper) orderDepth(ref string) int {
	if v, ok := b.bumpReferences[ref]; ok && v != ref {
		return b.orderDepth(v) + 1
	} else {
		return 0
	}
}

func (b *Bumper) DetermineBumpOrder() {
	var highest = 0
	var ordered = make(map[int][]string, len(b.bumpReferences))
	for ref, _ := range b.bumpReferences {
		depth := b.orderDepth(ref)
		if depth > highest {
			highest = depth
		}
		if v, ok := ordered[depth]; ok {
			ordered[depth] = append(v, ref)
		} else {
			ordered[depth] = []string{ref}
		}
	}

	for depth := 1; depth <= highest; depth = depth + 1 {
		if v, ok := ordered[depth]; ok {
			b.bumpOrder = append(b.bumpOrder, v...)
		}
	}
}

func MakeBumper(references []name.Reference, pin *tag.Level, latest bool) Bumper {
	var result = Bumper{make(map[string]string, len(references)),
		make([]string, 0, len(references)), false}

	for _, r := range references {
		if tagRef, ok := r.(name.Tag); ok {
			if nextTag, err := tag.GetNextVersion(tagRef, pin, latest); err == nil {
				result.bumpReferences[r.String()] = nextTag.String()
			} else {
				// skip references which do not have a next version
			}
		}
	}
	result.DetermineBumpOrder()
	return result
}
