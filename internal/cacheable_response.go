package internal

import (
	"bytes"
	"encoding/gob"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

const maxCacheLifetime = time.Duration(1<<31-1) * time.Second

var proxySetHeaders = []string{"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

type CacheableResponse struct {
	StatusCode    int
	HttpHeader    http.Header
	Body          []byte
	VariantHeader http.Header

	responseWriter http.ResponseWriter
	stasher        *stashingWriter
	headersWritten bool
}

func NewCacheableResponse(w http.ResponseWriter, maxBodyLength int) *CacheableResponse {
	return &CacheableResponse{
		StatusCode: http.StatusOK,
		HttpHeader: http.Header{},

		responseWriter: w,
		stasher:        NewStashingWriter(maxBodyLength, w),
	}
}

func CacheableResponseFromBuffer(b []byte) (CacheableResponse, error) {
	var cr CacheableResponse
	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&cr)

	return cr, err
}

func (c *CacheableResponse) ToBuffer() ([]byte, error) {
	c.Body = c.stasher.Body()

	var b bytes.Buffer
	encoder := gob.NewEncoder(&b)
	err := encoder.Encode(c)

	return b.Bytes(), err
}

func (c *CacheableResponse) Header() http.Header {
	return c.HttpHeader
}

func (c *CacheableResponse) Write(bytes []byte) (int, error) {
	if !c.headersWritten {
		c.WriteHeader(http.StatusOK)
	}
	return c.stasher.Write(bytes)
}

func (c *CacheableResponse) WriteHeader(statusCode int) {
	c.StatusCode = statusCode
	c.scrubHeaders()
	c.copyHeaders(c.responseWriter, false, c.StatusCode)
	c.headersWritten = true
}

func (c *CacheableResponse) Flush() {
	flusher, ok := c.responseWriter.(http.Flusher)
	if ok {
		flusher.Flush()
	}
}

func (c *CacheableResponse) CacheStatus() (bool, time.Time) {
	if c.stasher.Overflowed() {
		return false, time.Time{}
	}

	if c.StatusCode < 200 || c.StatusCode > 399 || c.StatusCode == http.StatusNotModified {
		return false, time.Time{}
	}

	for _, name := range varyNames(c.HttpHeader) {
		if name == "*" || slices.Contains(proxySetHeaders, name) {
			return false, time.Time{}
		}
	}

	cc := parseCacheControl(c.HttpHeader)

	if cc.malformed || !cc.has("public") || cc.has("private") || cc.has("no-store") || cc.has("no-cache") {
		return false, time.Time{}
	}

	lifetime, ok := cc.lifetime()
	if !ok || lifetime <= 0 {
		return false, time.Time{}
	}

	return true, time.Now().Add(lifetime)
}

func (c *CacheableResponse) WriteCachedResponse(w http.ResponseWriter, r *http.Request) {
	if c.wasNotModified(r) {
		c.copyHeaders(w, true, http.StatusNotModified)
	} else {
		c.copyHeaders(w, true, c.StatusCode)
		_, err := io.Copy(w, bytes.NewReader(c.Body))
		if err != nil {
			slog.Error("Error writing cached response body", "request_id", loggableRequestID(r), "error", err)
		}
	}
}

// Private

func (c *CacheableResponse) wasNotModified(r *http.Request) bool {
	requestEtag := c.HttpHeader.Get("Etag")
	if requestEtag == "" {
		return false
	}

	ifNoneMatch := strings.SplitSeq(r.Header.Get("If-None-Match"), ",")
	for etag := range ifNoneMatch {
		if strings.TrimSpace(etag) == requestEtag {
			return true
		}
	}

	return false
}

func (c *CacheableResponse) copyHeaders(w http.ResponseWriter, wasHit bool, statusCode int) {
	maps.Copy(w.Header(), c.HttpHeader)

	if wasHit {
		w.Header().Set("X-Cache", "hit")
	} else {
		w.Header().Set("X-Cache", "miss")
	}

	w.WriteHeader(statusCode)
}

func (c *CacheableResponse) scrubHeaders() {
	cacheable, _ := c.CacheStatus()

	if cacheable {
		c.HttpHeader.Del("Set-Cookie")
	}
}

type cacheControl struct {
	directives map[string][]string
	malformed  bool
}

func parseCacheControl(header http.Header) cacheControl {
	cc := cacheControl{directives: map[string][]string{}}
	for _, line := range header.Values("Cache-Control") {
		parts, balanced := splitOutsideQuotes(line, ',')
		if !balanced {
			cc.malformed = true
		}

		for _, directive := range parts {
			name, value, _ := strings.Cut(strings.TrimSpace(directive), "=")
			name = strings.ToLower(strings.TrimSpace(name))
			if name != "" {
				cc.directives[name] = append(cc.directives[name], unquote(strings.TrimSpace(value)))
			}
		}
	}
	return cc
}

func (cc cacheControl) has(name string) bool {
	_, ok := cc.directives[name]
	return ok
}

func (cc cacheControl) lifetime() (time.Duration, bool) {
	for _, name := range []string{"s-maxage", "max-age"} {
		values, ok := cc.directives[name]
		if !ok {
			continue
		}
		if len(values) != 1 {
			return 0, false
		}

		seconds, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil || seconds == 0 {
			return 0, false
		}
		if seconds > uint64(maxCacheLifetime/time.Second) {
			return maxCacheLifetime, true
		}
		return time.Duration(seconds) * time.Second, true
	}

	return 0, false
}

func unquote(value string) string {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}

func splitOutsideQuotes(s string, sep byte) (parts []string, balanced bool) {
	start := 0
	quoted := false

	for i := 0; i < len(s); i++ {
		switch {
		case quoted && s[i] == '\\' && i+1 < len(s):
			i++
		case s[i] == '"':
			quoted = !quoted
		case s[i] == sep && !quoted:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}

	return append(parts, s[start:]), !quoted
}

type stashingWriter struct {
	limit      int
	dest       io.Writer
	buffer     bytes.Buffer
	overflowed bool
}

func NewStashingWriter(limit int, dest io.Writer) *stashingWriter {
	return &stashingWriter{
		limit: limit,
		dest:  dest,
	}
}

func (w *stashingWriter) Write(p []byte) (int, error) {
	if w.buffer.Len()+len(p) > w.limit {
		w.overflowed = true
	} else {
		w.buffer.Write(p)
	}

	return w.dest.Write(p)
}

func (w *stashingWriter) Body() []byte {
	if w.overflowed {
		return nil
	}
	return w.buffer.Bytes()
}

func (w *stashingWriter) Overflowed() bool {
	return w.overflowed
}
