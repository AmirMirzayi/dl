package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

// Part defines the range and file path for a part of the file
type Part struct {
	ID       int
	Start    int64
	End      int64
	FilePath string
}

// download downloads a specific part of the file from `url`
func (part *Part) download(downloadURL string, wg *sync.WaitGroup, errChan chan error, byteChan chan readPart) {
	defer wg.Done()

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		errChan <- err
		return
	}

	// download specific bytes offset(start,end) of part
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start, part.End))

	client := &http.Client{Transport: NewMyTransport(byteChan, part.ID)}

	resp, err := client.Do(req)

	if err != nil {
		errChan <- err
		return
	}
	defer resp.Body.Close()

	// Create a file for this part
	out, err := os.Create(part.FilePath)
	if err != nil {
		errChan <- err
		return
	}
	defer out.Close()

	// Write the part content to the file
	if _, err = io.Copy(out, resp.Body); err != nil {
		errChan <- err
		return
	}
}

// mergeParts combines all part files into a single file
func mergeParts(parts []Part, outputFileName string) error {
	out, err := os.Create(outputFileName)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, part := range parts {
		in, err := os.Open(part.FilePath)
		if err != nil {
			return err
		}
		if _, err = io.Copy(out, in); err != nil {
			return err
		}
		if err = in.Close(); err != nil {
			fmt.Printf("failed to close file %v", err)
		}
	}
	return nil
}
