[![GoDoc](https://godoc.org/github.com/SoMuchForSubtlety/opendj?status.svg)](https://godoc.org/github.com/SoMuchForSubtlety/opendj)
[![Go Report Card](https://goreportcard.com/badge/github.com/SoMuchForSubtlety/opendj)](https://goreportcard.com/report/github.com/SoMuchForSubtlety/opendj)

# opendj
a simple library that makes it easy to implement a plug.dj clone

## example usage

```go
package main

import (
	"fmt"
	"github.com/SoMuchForSubtlety/opendj"
)

func main() {
	var dj opendj.Dj
	// add a handler that gets called when a new song plays
	dj.AddNewSongHandler(newSong)

	// create a QueueEntry
	// please don't actually do this manually
	var song opendj.Media
	song.Title = "BADBADNOTGOOD - CAN'T LEAVE THE NIGHT"
	song.URL = "https://www.youtube.com/watch?v=caY0MEok19I"
	song.Duration = 282000000000

	var entry opendj.QueueEntry
	entry.Media = song
	entry.Owner = "MyUsername"

	// add the entry to the queue
	dj.AddEntry(entry)

	// start playing to your favourite RTMP server
	dj.Play("http://example.org/rtmp/23rhwogvf984hgtw")
}

func newSong(entry opendj.QueueEntry) {
	fmt.Printf("now playing %s", entry.Media.Title)
}
```

