package active_storage

import (
	"io"
)

type ProcessedRepresentation struct {
	Reader      io.ReadCloser
	ContentType string
}

func (p *ProcessedRepresentation) Close() {
	p.Reader.Close()
}
