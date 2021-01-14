package indexer

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

var jobs int
var results int
var files int

var mutex sync.Mutex

func (catalog Catalog) BuildIndex(conn *pgxpool.Pool) error {

	start := time.Now()

	catalog.NdxJobs = make(chan *IndexFile, 5)
	catalog.NdxResults = make(chan *IndexFile, 5)

	done := make(chan bool)

	hostname, err := os.Hostname()
	if err != nil {
		log.Println("Error getting hostname")
		hostname = "localhost"
	}

	go catalog.CreateIndexerPool()
	go catalog.CreateResultsPool(conn, done)

	for _, ndxPath := range catalog.IndexPaths {
		ndxPath.Hostname = hostname
		err := catalog.BuildPathIndex(ndxPath.Path, ndxPath)
		if err != nil {
			log.Println("Error processing catalog path: " + ndxPath.Path)
		}

	}

	fmt.Println("Finished indexing")

	close(catalog.NdxJobs)

	<-done

	duration := time.Since(start)
	fmt.Printf("%v", duration)
	fmt.Println()
	fmt.Printf("files: %v, indexed: %v, results: %v", files, jobs, results)

	return nil
}

// retrieve the entries in this directory
// sort into directories and files
// send them to IndexCurrentPath
func (catalog Catalog) BuildPathIndex(basePath string, ndxPath IndexPath) error {
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
			catalog.BuildPathIndex(basePath, newNdxPath)
			continue
		}

		fullpath := filepath.Clean(ndxPath.Path + "/" + dirent.Name())
		relpath := strings.Replace(fullpath, basePath, "", 1)

		if ndxPath.ShouldIndexFile(dirent) {
			files++

			ndxfile := IndexFile{
				Hostname:     ndxPath.Hostname,
				Name:         dirent.Name(),
				FullPath:     fullpath,
				RelativePath: relpath,
				Size:         dirent.Size(),
				ModTime:      dirent.ModTime(),
				Cksum:        "",
				CksumBytes:   0} // initialize the CksumBytes to zero because it isn't calculated yet

			catalog.NdxJobs <- &ndxfile
		}
	}

	return nil
}

func (catalog Catalog) CreateResultsPool(conn *pgxpool.Pool, done chan bool) {
	var resultWaitGroup sync.WaitGroup

	for i := 0; i < catalog.ResultJobCount; i++ {
		go catalog.ProcessResultsWorker(conn, &resultWaitGroup)
		resultWaitGroup.Add(1)
	}

	resultWaitGroup.Wait()
	done <- true
}

func (catalog Catalog) CreateIndexerPool() {

	var jobWaitGroup sync.WaitGroup

	for i := 0; i < catalog.IndexJobCount; i++ {
		go catalog.ProcessIndexFileWorker(&jobWaitGroup)
		jobWaitGroup.Add(1)
	}

	jobWaitGroup.Wait()

	// let results know that no more are coming
	close(catalog.NdxResults)
}

func (catalog Catalog) ProcessResultsWorker(conn *pgxpool.Pool, wg *sync.WaitGroup) {

	for ndxFile := range catalog.NdxResults {

		mutex.Lock()
		results++
		mutex.Unlock()
		ndxFile.DisplayIndexFileToStdout()
		err := ndxFile.Store(conn)
		if err != nil {
			log.Printf("Error storing IndexFile: %v", err)
		}
	}

	cacheMutex.Lock()
	flushIndexCache(conn)
	cacheMutex.Unlock()

	wg.Done()
}

func (catalog Catalog) ProcessIndexFileWorker(wg *sync.WaitGroup) {

	for ndxFile := range catalog.NdxJobs {
		log.Printf("Indexing: %v", ndxFile.FullPath)
		mutex.Lock()
		jobs++
		mutex.Unlock()
		catalog.ProcessIndexFile(ndxFile)
		catalog.NdxResults <- ndxFile
	}
	wg.Done()

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

func (ndxFile IndexFile) Store(conn *pgxpool.Pool) error {
	// err := insertIndexFile(conn, ndxFile)
	err := addIndexFile(conn, ndxFile)
	if err != nil {
		return err
	}
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
	partial := false

	if cksumBytes != 0 {
		partial = true
	}

	fileHandle, err := os.Open(ndxFile.FullPath)

	if err != nil {
		log.Fatal("Failed to open file: " + ndxFile.FullPath)
		return err
	}
	defer fileHandle.Close()

	sha := sha256.New()

	if !partial {
		cksumBytes = ndxFile.Size
	}

	// if we've only checked part of the file
	// salt it with the actual filesize
	// it will do this even if the filesize is less than the requested number of bytes
	if partial {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(ndxFile.Size))
		sha.Write(b)
	}

	if _, err := io.CopyN(sha, fileHandle, cksumBytes); err != nil && err != io.EOF {
		log.Fatal(err)
		log.Fatal("Error computing sha256 " + ndxFile.FullPath)

		return err
	}

	ndxFile.Cksum = fmt.Sprintf("%x", sha.Sum(nil))
	ndxFile.CksumBytes = cksumBytes

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

		// check the full path
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
