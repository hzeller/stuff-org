Note, schema is now created directly in the code (see [dbbackend.go](../stuff/dbbackend.go))

# Content
The content collected here is merely a backup of our organization effort at Noisebridge, but
feel free to use it to play around (you are product shot images though, so things might look
a bit boring.

The [database dump](./initial-dump.db) found here is a database vendor independent dump that
can be handled with the excellent henplus JDBC commandline
utility ( https://github.com/neurolabs/henplus )

The sqlite-file.db is a binary file with the same content. You can use it with the
application by giving `--db_file=../db/sqlite-file.db`
