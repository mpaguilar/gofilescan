package indexer

import (
	"context"
	"fmt"
	"log"
	"time"

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

func addIndexFile(conn *pgxpool.Pool, ndxFile IndexFile) error {
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
