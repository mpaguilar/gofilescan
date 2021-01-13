package cmd

import (
	"filescan/indexer"
)

type DatabaseSettings struct {
	ConnectionString string
}

type AppSettings struct {
	Catalogs []indexer.Catalog
	Database DatabaseSettings
}

var appSettings AppSettings
