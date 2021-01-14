package indexer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/google/uuid"
)

/*
create table fileindex (
  Id Text,
  Hostname TEXT,
  Name Text,
  FullPath TEXT,
  RelativePath TEXT,
  Size INTEGER,
  ModTime TIMESTAMP WITH TIME ZONE,
  Cksum TEXT,
  CksumBytes INTEGER );
*/

var cachedNdx []IndexFile
var cacheMutex sync.Mutex

func addIndexFile(conn *pgxpool.Pool, ndxFile IndexFile) error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	cachedNdx = append(cachedNdx, ndxFile)
	if len(cachedNdx) >= 10 {
		flushIndexCache(conn)
	}

	return nil
}

func flushIndexCache(conn *pgxpool.Pool) {

	log.Printf("Flushing cache with %v entries", len(cachedNdx))
	fields := []string{"id", "hostname", "name", "fullpath", "relativepath", "size", "modtime", "cksum", "cksumbytes"}

	var data [][]interface{}
	for _, ndx := range cachedNdx {

		row := []interface{}{
			uuid.New().String(),
			ndx.Hostname,
			ndx.Name,
			ndx.FullPath,
			ndx.RelativePath,
			ndx.Size,
			ndx.ModTime.UTC().Format(time.RFC3339),
			ndx.Cksum,
			ndx.CksumBytes}

		data = append(data, row)
	}

	rows := pgx.CopyFromRows(data)
	copyCount, err := conn.CopyFrom(
		context.Background(),
		pgx.Identifier{"fileindex"},
		fields,
		rows)

	if err != nil {
		log.Fatalf("Error copying entries: %v", err)
	}

	log.Printf("Copied %v entries from cache", copyCount)

}

func insertIndexFile(conn *pgxpool.Pool, ndxFile IndexFile) error {
	insert_statement_fmt := "INSERT INTO fileindex(id, hostname, name, fullpath, relativepath, size, modtime, cksum, cksumbytes) VALUES ('%v', '%v', '%v', '%v', '%v', '%v', '%v', '%v', '%v')"

	stmt := fmt.Sprintf(insert_statement_fmt,
		uuid.New().String(),
		ndxFile.Hostname,
		ndxFile.Name,
		ndxFile.FullPath,
		ndxFile.RelativePath,
		ndxFile.Size,
		ndxFile.ModTime.UTC().Format(time.RFC3339),
		ndxFile.Cksum,
		ndxFile.CksumBytes)

	_, err := conn.Exec(context.Background(), stmt)
	if err != nil {
		log.Printf("Error inserting record: %v", err)
		return err
	}

	return nil
}
