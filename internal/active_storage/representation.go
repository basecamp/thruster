package active_storage

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	representationPathPrefix = "/rails/active_storage/representations"
)

var (
	representationPathPattern = regexp.MustCompile("^" + representationPathPrefix + `/(?:redirect|proxy)/([^/]+)/([^/]+)/(.*)`)
)

type Representation struct {
	BlobKey   string
	Variation string
	Filename  string
}

func IsRepresentationPath(path string) bool {
	return strings.HasPrefix(path, representationPathPrefix)
}

func ParseRepresentationFromPath(path string) (*Representation, error) {
	matches := representationPathPattern.FindStringSubmatch(path)

	if len(matches) < 4 {
		return nil, fmt.Errorf("path doesn't match ActiveStorage representation pattern: %s", path)
	}

	return &Representation{
		BlobKey:   matches[1],
		Variation: matches[2],
		Filename:  matches[3],
	}, nil
}

func (rep *Representation) Process(secret string) (*ProcessedRepresentation, error) {
	// TODO:
	// 1. Inspect the representation
	// 2. Generate a oreview, if required & possible
	// 3. Perform the requested transform
	// 4. Package the result into a ProcessedRepresentation

	return nil, fmt.Errorf("not implemented yet")
}
