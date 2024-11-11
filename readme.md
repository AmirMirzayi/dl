dl is a simple command-line file downloader with support multithreaded requests.
It don't require any dependencies and is not platform specific. It should work on linux, windows and macOS.
I preferred to use std libs over cool libraries like cobra, bubbletea, ...

## INSTALLATION
You can install this tool using go command "go install https://github.com/AmirMirzayi/dl" or download binary from https://github.com/AmirMirzayi/dl/releases.

## USAGE
Once you installed application, run to download file using command "dl https://file-examples.com/wp-content/storage/2017/04/file_example_MP4_1920_18MG.mp4" in Terminal.

## OPTIONS
-h      print the help text for available options
-o      specify download file save path

### Todo
- [X] optional download file-name by user input
- [X] save temp parts by given user's file-name
- [ ] remove temp parts if download fails
- [ ] configurable http client(proxy, timeout, download path and workers)
- [ ] retry failed download
- [ ] store downloaded temp-files to resume them later(if server support)
- [ ] download single parted file, if server doesn't support concurrent request
- [ ] store link to start download process later
- [ ] decrease download speed to consume less i/o and network bandwidth
- [ ] build cross-platform gui application
- [ ] dynamic download workers count
- [ ] add graceful shutdown
- [X] cancellable download threads