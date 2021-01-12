package indexer

import (
	"time"
)

type IndexPath struct {
	Hostname string
	Path     string
	Exclude  []string
	Include  []string
}

type Catalog struct {
	Name           string
	CksumKBytes    int64
	IndexPaths     []IndexPath
	NdxJobs        chan *IndexFile
	NdxResults     chan *IndexFile
	IndexJobCount  int
	ResultJobCount int
}

// IndexFile stores the exact number of bytes
// in case the whole file is used
type IndexFile struct {
	// the machine this was run on
	Hostname string
	// just the filename
	Name string
	// the full path to the file on this machine
	FullPath string
	// the path from the root, for directory comparison
	RelativePath string
	// the size as reported by the OS
	Size int64
	// when the file was last modified
	ModTime time.Time
	// sha256sum
	Cksum string
	// number of bytes used to compute Cksum
	CksumBytes int64
}
