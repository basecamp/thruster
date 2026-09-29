package internal

import (
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

func (v *Variant) CacheKey() RequestKey {
	vary := make([]string, len(v.headerNames))
	for i, name := range v.headerNames {
		vary[i] = name + "=" + v.r.Header.Get(name)
	}

	return RequestKey{
		Method: strings.Clone(v.r.Method),
		Host:   strings.Clone(v.r.Host),
		Path:   strings.Clone(v.r.URL.Path),
		Query:  v.r.URL.Query().Encode(),
		Vary:   strings.Join(vary, "\n"),
	}
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
