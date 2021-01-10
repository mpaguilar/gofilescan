package indexer

import (
	"sync"
	"time"
)

type IndexPath struct {
	Hostname string
	Path     string
	Exclude  []string
	Include  []string
}

type Catalog struct {
	Name                string
	CksumKBytes         int64
	IndexPaths          []IndexPath
	NdxJobs             chan *IndexFile
	NdxJobWaitGroup     *sync.WaitGroup
	NdxResults          chan *IndexFile
	NdxResultsWaitGroup *sync.WaitGroup
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
