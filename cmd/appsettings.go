package cmd

import (
	"filescan/indexer"
)

type AppSettings struct {
	Catalogs []indexer.Catalog
}

var appSettings AppSettings
