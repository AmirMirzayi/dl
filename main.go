package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	args := os.Args

	// 1st args is application running path, 2nd(or last one) is download url
	argURL := args[len(args)-1]

	if len(args) < 2 {
		fmt.Println("download link argument is required")
		return
	}

	argFileName := flag.String("o", "", "output file name")
	flag.Parse()

	fileName := filepath.Base(argURL)
	if *argFileName != "" {
		// trim last dot character to cover mistyping
		fileName = strings.TrimRight(*argFileName, ".")
		// save fileName by download link's file type if arg hasn't
		if filepath.Ext(fileName) == "" {
			fileName += filepath.Ext(filepath.Base(argURL))
		}
	}

	outputFileName, err := url.QueryUnescape(fileName)
	if err != nil {
		fmt.Println("download link is not valid :(")
		return
	}

	startingTime := time.Now()

	// Step 1: Get file size
	resp, err := http.Head(argURL)
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

				progressBarWidth := 25 // should be divisible to 100
				progressBar := strings.Repeat("", 100-downloadedPercent/(100/progressBarWidth))
				progressBar += strings.Repeat("█", downloadedPercent/(100/progressBarWidth))

				fmt.Printf("[%-*s] #%d - %d%% | speed: %s | %s of %s ✓\n",
					progressBarWidth,
					progressBar,
					i+1,
					downloadedPercent,
					convertByteSizeToHumanReadable(downloadSpeed[i]),
					convertByteSizeToHumanReadable(float64(partTotalByteReceived[i])),
					convertByteSizeToHumanReadable(float64(partByteSize)),
				)
			}
		}
	}()

	for i, part := range parts {
		wg.Add(1)
		go part.download(argURL, &wg, errChan, receivingByteChans[i])
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
		convertByteSizeToHumanReadable(float64(fileSize)),
		filepath.Dir(outputFilePath),
		downloadTakenTime,
	)
}
