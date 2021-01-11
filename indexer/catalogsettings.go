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
	Hostname   string
	Name       string
	FullPath   string
	Size       int64
	ModTime    time.Time
	Cksum      string
	CksumBytes int64
}
