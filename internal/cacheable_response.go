package internal

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var (
	publicExp   = regexp.MustCompile(`\bpublic\b`)
	noCacheExpt = regexp.MustCompile(`\bno-cache\b`)
	sMaxAgeExp  = regexp.MustCompile(`\bs-max-age=(\d+)\b`)
	maxAgeExp   = regexp.MustCompile(`\bmax-age=(\d+)\b`)
)

type CacheableResponse struct {
	StatusCode int
	HttpHeader http.Header
	Body       []byte

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
	c.copyHeaders(c.responseWriter, false)
	c.headersWritten = true
}

func (c *CacheableResponse) CacheStatus() (bool, time.Time) {
	if c.stasher.Overflowed() {
		return false, time.Time{}
	}

	if c.StatusCode < 200 || c.StatusCode > 399 || c.StatusCode == http.StatusNotModified {
		return false, time.Time{}
	}

	// TODO: support Vary header properly
	if c.HttpHeader.Get("Vary") != "" {
		return false, time.Time{}
	}

	cc := c.HttpHeader.Get("Cache-Control")

	if !publicExp.MatchString(cc) || noCacheExpt.MatchString(cc) {
		return false, time.Time{}
	}

	matches := sMaxAgeExp.FindStringSubmatch(cc)
	if len(matches) != 2 {
		matches = maxAgeExp.FindStringSubmatch(cc)
	}
	if len(matches) != 2 {
		return false, time.Time{}
	}

	maxAge, err := strconv.Atoi(matches[1])
	if err != nil || maxAge <= 0 {
		return false, time.Time{}
	}

	return true, time.Now().Add(time.Duration(maxAge) * time.Second)
}

func (c *CacheableResponse) WriteCachedResponse(w http.ResponseWriter) {
	c.copyHeaders(w, true)
	io.Copy(w, bytes.NewReader(c.Body))
}

// Private

func (c *CacheableResponse) copyHeaders(w http.ResponseWriter, wasHit bool) {
	for k, v := range c.HttpHeader {
		w.Header()[k] = v
	}

	if wasHit {
		w.Header().Set("X-Cache", "hit")
	} else {
		w.Header().Set("X-Cache", "miss")
	}

	w.WriteHeader(c.StatusCode)
}

func (c *CacheableResponse) scrubHeaders() {
	cacheable, _ := c.CacheStatus()

	if cacheable {
		c.HttpHeader.Del("Set-Cookie")
	}
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
