package model

import (
	"hash/fnv"
	"strconv"

	"go.yaml.in/yaml/v3"
)

// Rev returns a short fingerprint of v. Edit forms carry the fingerprint of
// the data they were rendered from, so a save can detect that the same data
// changed elsewhere in the meantime.
func Rev(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	h := fnv.New64a()
	h.Write(b)
	return strconv.FormatUint(h.Sum64(), 36)
}
