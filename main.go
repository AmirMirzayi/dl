package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Part defines the range and file path for a part of the file
type Part struct {
	Start    int64
	End      int64
	FilePath string
}

// TrackReader tracks bytes read
type TrackReader struct {
	Reader         io.ReadCloser
	TotalBytesRead int
	ByteRead,
	ByteReadPerSecond chan<- int
}

// Read wraps Read function to count bytes
func (t *TrackReader) Read(p []byte) (int, error) {
	n, err := t.Reader.Read(p)
	t.ByteRead <- n
	t.TotalBytesRead += n
	return n, err
}

// Close closes the wrapped reader
func (t *TrackReader) Close() error {
	return t.Reader.Close()
}

type myTransport struct {
	Transport http.RoundTripper
	byteRead,
	byteReadPerSecond chan<- int
}

func NewMyTransport(byteRead, byteReadPerSecond chan<- int) *myTransport {
	return &myTransport{
		Transport:         http.DefaultTransport,
		byteRead:          byteRead,
		byteReadPerSecond: byteReadPerSecond,
	}
}

func (t *myTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	trackReader := &TrackReader{
		Reader:            resp.Body,
		ByteRead:          t.byteRead,
		ByteReadPerSecond: t.byteReadPerSecond,
	}
	resp.Body = trackReader
	go func() {
		tick := time.NewTicker(time.Second)
		for range tick.C {
			trackReader.ByteReadPerSecond <- trackReader.TotalBytesRead
			trackReader.TotalBytesRead = 0
		}
	}()

	return resp, nil
}

// download downloads a specific part of the file from `url`
func (part *Part) download(url string, wg *sync.WaitGroup, errChan chan<- error, byteChan, byteReadPerSecond chan<- int) {
	defer wg.Done()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		errChan <- err
		return
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start, part.End))

	cl := &http.Client{Transport: NewMyTransport(byteChan, byteReadPerSecond)}

	resp, err := cl.Do(req)

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
	_, err = io.Copy(out, resp.Body)
	if err != nil {
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
		_, err = io.Copy(out, in)
		if err != nil {
			return err
		}
		_ = in.Close()
	}
	return nil
}

func main() {
	startingTime := time.Now()

	args := os.Args

	if len(args) < 2 {
		fmt.Println("download link argument is required")
		return
	}

	url := args[1]
	outputFileName := filepath.Base(url)
	outputFileName = strings.ReplaceAll(outputFileName, "%20", " ")

	// Step 1: Get file size
	resp, err := http.Head(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	fileSize := resp.ContentLength

	// Step 2: Define number of parts and divide file size
	numParts := 4
	partSize := fileSize / int64(numParts)
	var parts []Part

	for i := 0; i < numParts; i++ {
		start := partSize * int64(i)
		end := start + partSize - 1
		if i == numParts-1 {
			end = fileSize
		}
		part := Part{
			Start:    start,
			End:      end,
			FilePath: fmt.Sprintf("part-%d.tmp", i),
		}
		parts = append(parts, part)
	}

	// Step 3: Download each part concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, numParts)

	byteChans := make([]chan int, numParts)
	bpsChans := make([]chan int, numParts)

	totalByteReceived := make([]int, numParts)
	downloadSpeed := make([]int, numParts)

	for i := 0; i < numParts; i++ {
		byteChans[i] = make(chan int)
		bpsChans[i] = make(chan int)
		go func(j int) {
			for {
				go func(k int) {
					totalByteReceived[k] += <-byteChans[k]
				}(j)
				go func(k int) {
					downloadSpeed[j] = <-bpsChans[j]
				}(j)
			}
		}(i)
	}

	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		for range tick.C {
			fmt.Print("\033[H\033[2J")

			for i := 0; i < numParts; i++ {
				partByteSize := parts[i].End - parts[i].Start
				var completedByteSize int
				if totalByteReceived[i] != 0 {
					completedByteSize = (totalByteReceived[i] * 100) / int(partByteSize)
				}
				fmt.Printf("part %d - %d%% | speed: %s | %s of %s completed.\n",
					i,
					completedByteSize,
					convertByteSizeToHumanReadable(downloadSpeed[i]),
					convertByteSizeToHumanReadable(totalByteReceived[i]),
					convertByteSizeToHumanReadable(int(partByteSize)),
				)
			}
		}
	}()
	for i, part := range parts {
		wg.Add(1)
		go part.download(url, &wg, errChan, byteChans[i], bpsChans[i])
	}
	wg.Wait()
	close(errChan)

	end := time.Since(startingTime)

	// Check for any errors in the download
	if len(errChan) > 0 {
		for err = range errChan {
			fmt.Println("Error during download:", err)
		}
		return
	}

	outputFilePath := outputFileName
	dir, err := os.UserHomeDir()
	if err == nil {
		outputFilePath = filepath.Join(dir, outputFilePath)
	}
	// Step 4: Merge parts into a single file
	if err = mergeParts(parts, outputFilePath); err != nil {
		fmt.Println("Error merging parts:", err)
		return
	}

	// Clean up part files
	for _, part := range parts {
		_ = os.Remove(part.FilePath)
	}

	fmt.Printf("Download completed successfully in %s at %s.\n", end, outputFilePath)
}

var sizes = []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}

const base = 1024

func convertByteSizeToHumanReadable(sizeInByte int) string {
	unitsLimit := len(sizes)
	i := 0

	size := float32(sizeInByte)
	for size >= base && i < unitsLimit {
		size = size / base
		i++
	}
	f := "%.0f %s"
	if i > 1 {
		f = "%.2f %s"
	}
	return fmt.Sprintf(f, size, sizes[i])
}
