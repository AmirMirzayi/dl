dl is simple command-line file downloader with support multi-threaded requests.

## Uses
You can install this tool using go command "go install https://github.com/AmirMirzayi/dl" or ddownload binary from Release.
once you installed application, run to download file using command "dl https://file-examples.com/wp-content/storage/2017/04/file_example_MP4_1920_18MG.mp4" in Terminal.


### Todo
- [ ] optional download file-name by user input
- [X] save temp parts by given user's file-name
- [ ] remove temp parts if download fails
- [ ] configurable http client(proxy, timeout, download path)
- [ ] retry failed download
- [ ] store downloaded temp-files to resume them later(if server support)
- [ ] download single parted file, if server doesn't support concurrent request
- [ ] store link to start download process later
- [ ] decrease download speed to consume less i/o and network bandwidth
- [ ] build cross platform gui application