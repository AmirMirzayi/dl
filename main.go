package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

// download downloads a specific part of the file from `url`
func (part *Part) download(downloadURL string, wg *sync.WaitGroup, errChan chan error, byteChan chan int) {
	defer wg.Done()

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		errChan <- err
		return
	}

	// download specific bytes offset(start,end) of part
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start, part.End))

	client := &http.Client{Transport: NewMyTransport(byteChan)}

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

func main() {
	startingTime := time.Now()

	args := os.Args

	if len(args) < 2 {
		fmt.Println("download link argument is required")
		return
	}

	downloadURL := args[1]
	outputFileName := filepath.Base(downloadURL)
	outputFileName, err := url.QueryUnescape(outputFileName)
	if err != nil {
		fmt.Println("download link is not valid :(")
		return
	}

	// Step 1: Get file size
	resp, err := http.Head(downloadURL)
	if err != nil {
		fmt.Println(err)
		return
	}

	fileSize := resp.ContentLength

	outputFilePath := outputFileName
	dir, err := os.UserHomeDir()
	if err == nil {
		outputFilePath = filepath.Join(dir, outputFilePath)
	}

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
			FilePath: fmt.Sprintf("%s-part-%d.tmp", outputFileName, i),
		}
		parts = append(parts, part)
	}

	// Step 3: Download each part concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, numParts)

	receivingByteChans := make([]chan int, numParts)
	partTotalByteReceived := make([]int, numParts)
	downloadSpeed := make([]float64, numParts)
	lastTimeByteReceived := make([]time.Time, numParts)

	for i := 0; i < numParts; i++ {
		receivingByteChans[i] = make(chan int)
		go func(j int) {
			for {
				receivedByteSize := <-receivingByteChans[j]
				partTotalByteReceived[j] += receivedByteSize
				downloadSpeed[j] = float64(receivedByteSize) / time.Since(lastTimeByteReceived[j]).Seconds()
				lastTimeByteReceived[j] = time.Now()
			}
		}(i)
	}

	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		for range tick.C {
			fmt.Print("\033[H\033[2J")

			for i := 0; i < numParts; i++ {
				partByteSize := parts[i].End - parts[i].Start
				var downloadedPercent int
				if partTotalByteReceived[i] != 0 {
					downloadedPercent = (partTotalByteReceived[i] * 100) / int(partByteSize)
				}
				fmt.Printf("part %d - %d%% | speed: %s | %s of %s completed.\n",
					i+1,
					downloadedPercent,
					convertFloatByteSizeToHumanReadable(currentSpeed[i]),
					convertByteSizeToHumanReadable(partTotalByteReceived[i]),
					convertByteSizeToHumanReadable(int(partByteSize)),
				)
			}
		}
	}()

	for i, part := range parts {
		wg.Add(1)
		go part.download(downloadURL, &wg, errChan, receivingByteChans[i])
	}
	wg.Wait()
	close(errChan)

	downloadTakenTime := time.Since(startingTime)

	// Check for any errors in the download
	if len(errChan) > 0 {
		for err = range errChan {
			fmt.Println("Error during download:", err)
		}
		return
	}

	// Step 4: Merge parts into a single file
	if err = mergeParts(parts, outputFilePath); err != nil {
		fmt.Println("Error merging parts:", err)
		return
	}

	// Clean up part files
	for _, part := range parts {
		if err = os.Remove(part.FilePath); err != nil {
			fmt.Printf("failed to remove downloaded temp part: %s\n", part.FilePath)
		}
	}

	fmt.Printf("%s (%s) Downloaded on %s in %s.",
		outputFileName,
		convertByteSizeToHumanReadable(int(fileSize)),
		filepath.Dir(outputFilePath),
		downloadTakenTime,
	)
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

func convertFloatByteSizeToHumanReadable(size float64) string {
	unitsLimit := len(sizes)
	i := 0

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
