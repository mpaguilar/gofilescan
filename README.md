A simple file checksum utility. It doesn't really do all that much, created to learn about Go.

It will scan a configurable amount of the file, and create a checksum of those bytes salted with the actual length of the file. If "0" is specified, then it will scan the entire file and will not use a salt.

The file details are then stored into Postgres. I tried using CrateDB at first, but I kept getting errors trying to do a bulk COPY FROM.

Currently, it expects an empty table - no checking for existing entries is done.

It accumulates the results of 1000 files before flushing to the database. Without any database, it took about a second to index 32K files, using the first 4K bytes. It took about 4 seconds to read through 3Gb - this is very dependent on how fast the I/O is on the target drives.

