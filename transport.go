package main

import (
	"io"
	"net/http"
)

type readPart struct {
	partID   int
	byteSize int
}

// TrackReader tracks bytes read
type TrackReader struct {
	Reader   io.ReadCloser
	ByteRead chan readPart
	partID   int
}

// Read wraps Read function to count bytes
func (t *TrackReader) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	t.ByteRead <- readPart{partID: t.partID, byteSize: n}
	return n, err
}

// Close closes the wrapped reader
func (t *TrackReader) Close() error {
	return t.Reader.Close()
}

type myTransport struct {
	Transport http.RoundTripper
	byteRead  chan readPart
	partID    int
}

func NewMyTransport(byteRead chan readPart, partID int) *myTransport {
	return &myTransport{
		Transport: http.DefaultTransport,
		byteRead:  byteRead,
		partID:    partID,
	}
}

func (t *myTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	trackReader := &TrackReader{
		Reader:   resp.Body,
		ByteRead: t.byteRead,
		partID:   t.partID,
	}
	resp.Body = trackReader

	return resp, nil
}
