package indexer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var jobs int
var results int

func (catalog Catalog) BuildIndex() error {

	start := time.Now()

	var wg sync.WaitGroup
	var resWaitGroup sync.WaitGroup

	catalog.NdxJobWaitGroup = &wg
	catalog.NdxResultsWaitGroup = &resWaitGroup

	catalog.NdxJobs = make(chan *IndexFile)
	catalog.NdxResults = make(chan *IndexFile)

	hostname, err := os.Hostname()
	if err != nil {
		log.Println("Error getting hostname")
		hostname = "localhost"
	}

	for i := 0; i < 5; i++ {
		go catalog.ProcessIndexFileWorker()
		go catalog.ProcessResultsWorker()
	}

	for _, ndxPath := range catalog.IndexPaths {
		ndxPath.Hostname = hostname
		err := catalog.BuildPathIndex(ndxPath)
		if err != nil {
			log.Println("Error processing catalog path: " + ndxPath.Path)
		}

	}

	wg.Wait()
	resWaitGroup.Wait()

	duration := time.Since(start)
	fmt.Printf("%v", duration)
	fmt.Println()
	fmt.Printf("jobs: %v, results: %v", jobs, results)

	return nil
}

// retrieve the entries in this directory
// sort into directories and files
// send them to IndexCurrentPath
func (catalog Catalog) BuildPathIndex(ndxPath IndexPath) error {
	var err error

	cleanPath := filepath.Clean(ndxPath.Path)
	cleanPath = strings.Replace(cleanPath, "\\", "/", -1)

	log.Println("Processing directory: " + cleanPath)

	dirEnts, err := ioutil.ReadDir(cleanPath)
	if err != nil {
		return err
	}

	for _, dirent := range dirEnts {
		if dirent.IsDir() {
			newNdxPath := ndxPath
			newNdxPath.Path = ndxPath.Path + "/" + dirent.Name()
			catalog.BuildPathIndex(newNdxPath)
			continue
		}

		fullpath := filepath.Clean(ndxPath.Path + "/" + dirent.Name())

		if ndxPath.ShouldIndexFile(dirent) {

			ndxfile := IndexFile{
				ndxPath.Hostname,
				dirent.Name(),
				fullpath,
				dirent.Size(),
				dirent.ModTime(),
				"",
				0} // initialize the CksumBytes to zero because it isn't calculated yet

			catalog.NdxJobWaitGroup.Add(1)
			catalog.NdxJobs <- &ndxfile

		}
	}

	return nil
}

func (catalog Catalog) ProcessIndexFileWorker() {
	for ndxFile := range catalog.NdxJobs {
		catalog.ProcessIndexFile(ndxFile)

		catalog.NdxResultsWaitGroup.Add(1)
		jobs++
		catalog.NdxResults <- ndxFile

		catalog.NdxJobWaitGroup.Done()
	}
}

func (catalog Catalog) ProcessResultsWorker() {
	for ndxFile := range catalog.NdxResults {
		results++
		ndxFile.DisplayIndexFileToStdout()

		catalog.NdxResultsWaitGroup.Done()
	}
}

func (catalog Catalog) ProcessIndexFile(ndxFile *IndexFile) error {
	var err error
	// ndxFile.Sha256Sum(catalog.CksumBytes * 1024)
	cksumBytes := catalog.CksumKBytes * 1024
	if cksumBytes == 0 {
		cksumBytes = ndxFile.Size
	}

	cksumtmp, err := CalcSha256(ndxFile.FullPath, cksumBytes)
	if err != nil {
		return err
	}

	ndxFile.Cksum = fmt.Sprintf("%x", cksumtmp)
	ndxFile.CksumBytes = cksumBytes

	return nil
}

func (ndxFile IndexFile) DisplayIndexFileToStdout() error {
	fmt.Printf("%v %v", ndxFile.FullPath, ndxFile.Cksum)
	fmt.Println()

	return nil
}

func CalcSha256(fullPath string, cksumBytes int64) ([]byte, error) {
	var err error

	fileHandle, err := os.Open(fullPath)

	if err != nil {
		log.Fatal("Failed to open file: " + fullPath)
		return nil, err
	}
	defer fileHandle.Close()

	sha := sha256.New()

	if _, err := io.CopyN(sha, fileHandle, cksumBytes); err != nil && err != io.EOF {
		log.Fatal(err)
		log.Fatal("Error computing sha256 " + fullPath)

		return nil, err
	}

	return sha.Sum(nil), nil
}

func (ndxFile *IndexFile) Sha256Sum(cksumBytes int64) error {

	var err error

	fileHandle, err := os.Open(ndxFile.FullPath)

	if err != nil {
		log.Fatal("Failed to open file: " + ndxFile.FullPath)
		return err
	}
	defer fileHandle.Close()

	sha := sha256.New()

	if cksumBytes == 0 {
		cksumBytes = ndxFile.Size
	}

	if _, err := io.CopyN(sha, fileHandle, cksumBytes); err != nil && err != io.EOF {
		log.Fatal(err)
		log.Fatal("Error computing sha256 " + ndxFile.FullPath)

		return err
	}
	ndxFile.Cksum = fmt.Sprintf("%x", sha.Sum(nil))

	return nil
}

func (ndxPath *IndexPath) ShouldIndexFile(fileNfo os.FileInfo) bool {

	var matched bool
	for _, ex := range ndxPath.Exclude {
		matched, _ = filepath.Match(ex, fileNfo.Name())
		// check just the fileNfo
		if matched {
			return false
		}

		// check the whole path
		fullpath := filepath.Join(ndxPath.Path, fileNfo.Name())
		matched, _ = filepath.Match(ex, fullpath)
		if matched {
			return false
		}
	}

	// if nothing is specifically included
	// then everything is included
	if len(ndxPath.Include) > 0 {
		for _, in := range ndxPath.Include {
			matched, _ = filepath.Match(in, filepath.Base(fileNfo.Name()))
			if matched {
				break
			}
		}
		if matched {
			return true
		} else {
			return false
		}
	} else {
		return true
	}
}
