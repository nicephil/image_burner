package oakUtility

/*
 * this is from https://golangcode.com/download-a-file-from-a-url/
 */
import (
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"net/http"
	"os"
	"strings"
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer
// interface and we can pass this into io.TeeReader() which will report progress on each
// write cycle.
type WriteCounter struct {
    Total uint64
    Prefix_txt string
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

func (wc WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", len(wc.Prefix_txt)+16))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\r%s%s (%s)", wc.Prefix_txt, humanize.Bytes(wc.Total), wc.Total)
}


// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory. We pass an io.TeeReader
// into Copy() to report progress on the download.
func DownloadFile(filepath string, url string, progress bool, prefix string) error {

	// Create the file, but give it a tmp file extension, this means we won't overwrite a
	// file until it's downloaded, but we'll remove the tmp extension once downloaded.
	out, err := os.Create(filepath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

    if progress == true {
	    counter := &WriteCounter{Prefix_txt: prefix}
	    _, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	    fmt.Print("\n")
    } else {
	    _, err = io.Copy(out, resp.Body)
    }
	if err != nil {
	    return err
	}

	err = os.Rename(filepath+".tmp", filepath)
	if err != nil {
		return err
	}

	return nil
}
