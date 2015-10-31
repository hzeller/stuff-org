Note, schema is now created directly in the code (see [dbbackend.go](../stuff/dbbackend.go))

# Content
The content collected here is merely a backup of our organization effort at Noisebridge, but
feel free to use it to play around (you are missing product shot images though, so things might
look a bit boring).

# Manual poking in the database
I suggest to use the excellent henplus JDBC commandline
utility ( https://github.com/neurolabs/henplus )

The [database dump](./initial-db.dump) found here is a database vendor independent dump that
can be handled with henplus.

Get a jdbc driver at https://bitbucket.org/xerial/sqlite-jdbc/downloads and copy it in a
`stuff-org/.henplus/lib` directory in the project-root (that you have to create).

```
henplus -J jdbc:sqlite:sqlite-file.db
```

(use `dump-in` to read the dump (type `help dump-in` on the HenPlus shell).

The sqlite-file.db is a SQLite binary database file with the same content already dumped in
for convenience.
You can use it with the application by passing it in with the `--db-file` flag.
