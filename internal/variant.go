package internal

import (
	"hash/fnv"
	"net/http"
	"slices"
	"strings"
)

type Variant struct {
	r           *http.Request
	headerNames []string
}

func NewVariant(r *http.Request) *Variant {
	return &Variant{r: r}
}

func (v *Variant) SetResponseHeader(header http.Header) {
	v.headerNames = v.parseVaryHeader(header)
}

func (v *Variant) CacheKey() CacheKey {
	hash := fnv.New64()
	hash.Write([]byte(v.r.Method))
	hash.Write([]byte(v.r.URL.Path))
	hash.Write([]byte(v.r.URL.Query().Encode()))

	for _, name := range v.headerNames {
		hash.Write([]byte(name + "=" + v.r.Header.Get(name)))
	}

	return CacheKey(hash.Sum64())
}

func (v *Variant) Matches(responseHeader http.Header) bool {
	for _, name := range v.headerNames {
		if responseHeader.Get(name) != v.r.Header.Get(name) {
			return false
		}
	}
	return true
}

func (v *Variant) VariantHeader() http.Header {
	requestHeader := http.Header{}
	for _, name := range v.headerNames {
		requestHeader.Set(name, v.r.Header.Get(name))
	}
	return requestHeader
}

// Private

func (v *Variant) parseVaryHeader(responseHeader http.Header) []string {
	list := responseHeader.Get("Vary")
	if list == "" {
		return []string{}
	}

	names := strings.Split(list, ",")
	for i, name := range names {
		names[i] = http.CanonicalHeaderKey(strings.TrimSpace(name))
	}
	slices.Sort(names)

	return names
}
