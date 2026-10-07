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
		values, present := v.r.Header[name]
		if present {
			vary[i] = name + "=" + strings.Join(values, "\x00")
		} else {
			vary[i] = name
		}
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
		responseValues, responsePresent := responseHeader[name]
		requestValues, requestPresent := v.r.Header[name]

		if responsePresent != requestPresent || !slices.Equal(responseValues, requestValues) {
			return false
		}
	}
	return true
}

func (v *Variant) VariantHeader() http.Header {
	requestHeader := http.Header{}
	for _, name := range v.headerNames {
		if values, present := v.r.Header[name]; present {
			requestHeader[name] = slices.Clone(values)
		}
	}
	return requestHeader
}

// Private

func (v *Variant) parseVaryHeader(responseHeader http.Header) []string {
	names := varyNames(responseHeader)
	slices.Sort(names)
	return slices.Compact(names)
}

func varyNames(header http.Header) []string {
	names := []string{}
	for _, line := range header.Values("Vary") {
		for name := range strings.SplitSeq(line, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				names = append(names, http.CanonicalHeaderKey(name))
			}
		}
	}
	return names
}
