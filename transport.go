package main

import (
	"io"
	"net/http"
)

// TrackReader tracks bytes read
type TrackReader struct {
	Reader   io.ReadCloser
	ByteRead chan int
}

// Read wraps Read function to count bytes
func (t *TrackReader) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	t.ByteRead <- n
	return n, err
}

// Close closes the wrapped reader
func (t *TrackReader) Close() error {
	return t.Reader.Close()
}

type myTransport struct {
	Transport http.RoundTripper
	byteRead  chan int
}

func NewMyTransport(byteRead chan int) *myTransport {
	return &myTransport{
		Transport: http.DefaultTransport,
		byteRead:  byteRead,
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
	}
	resp.Body = trackReader

	return resp, nil
}
