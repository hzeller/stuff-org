Keeping track of stuff
----------------------

Mostly just to organize electronic components at home and at hackerspace.

Nothing to see here yet, work in progress, quick hack.
With a neat useful search though :)

Uses SQLite to keep data in one file.

```
go get github.com/mattn/go-sqlite3
```

Beware, I am just playing around with go and a simple http server to learn things;
so this doesn't use a web framework of any kind, only what comes stock with the
golang libraries.

Also, I don't know any CSS or JavaScript...

Provides

- Enter form to enter details found in boxes with a given ID
- Search form with search-as-you-type in an legitimate use of JSON ui :)
- Some search API returning JSON formatted results.

## API

Next to a web-UI, this provides as well a search API with JSON response
to be integrated in other apps, e.g. slack

### Sample query
```
http://pegasus:3000/api/search?q=fet
```

Optional URL-parameter `count=42` to limit the number of results (default: 100).

### Sample response
```json
{
  "link": "/search#fet",
  "components": [
    {
      "id": 42,
      "equiv_set": 42,
      "value": "BUK9Y16-60E",
      "category": "Mosfet",
      "description": "Mosfet N-channel, 60V, 53A, 12.1mOhm\nSOT669",
      "quantity": "25",
      "notes": "",
      "datasheet_url": "http://www.nxp.com/documents/data_sheet/BUK9Y15-60E.pdf",
      "footprint": "LFPAK56",
      "img": "/img/42"
    },
    {
      "id": 76,
      "equiv_set": 76,
      "value": "BUK9Y4R4-40E",
      "category": "Mosfet",
      "description": "N-Channel MOSFET, 40V, 4.4mOhm@5V, 3.7mOhm@10V",
      "quantity": "4",
      "notes": "",
      "datasheet_url": "http://www.nxp.com/documents/data_sheet/BUK9Y4R4-40E.pdf",
      "footprint": "LFPAK56",
      "img": "/img/76"
    }
  ]
}
```
