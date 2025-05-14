package e2e

import (
	"fmt"
	"net"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/SoMuchForSubtlety/opendj"
	"github.com/nareix/joy5/format/rtmp"
)

func TestMain(m *testing.M) {
	server := rtmp.NewServer()

	publishers := make(map[string]*rtmp.Conn)
	players := make(map[string][]*rtmp.Conn)
	var lock sync.Mutex

	server.HandleConn = func(c *rtmp.Conn, nc net.Conn) {
		defer nc.Close()

		if c.Publishing {
			lock.Lock()
			publishers[c.URL.Path] = c
			lock.Unlock()

			defer func() {
				lock.Lock()
				delete(publishers, c.URL.Path)
				lock.Unlock()
			}()

			for {
				pkt, err := c.ReadPacket()
				if err != nil {
					break
				}

				lock.Lock()
				for _, player := range players[c.URL.Path] {
					player.WritePacket(pkt)
				}
				lock.Unlock()
			}
		} else {
			lock.Lock()
			if _, ok := players[c.URL.Path]; !ok {
				players[c.URL.Path] = []*rtmp.Conn{c}
			} else {
				players[c.URL.Path] = append(players[c.URL.Path], c)
			}
			lock.Unlock()

			defer func() {
				lock.Lock()
				for i, player := range players[c.URL.Path] {
					if player == c {
						players[c.URL.Path] = slices.Delete(players[c.URL.Path], i, i+1)
						break
					}
				}
				lock.Unlock()
			}()

			<-c.CloseNotify()
		}
	}

	listener, err := net.Listen("tcp", ":1935")
	if err != nil {
		panic(fmt.Errorf("Failed to start listener: %v", err))
	}

	go func() {
		for {
			nc, err := listener.Accept()
			if err != nil {
				panic(fmt.Errorf("Failed to accept connection: %v", err))
			}
			go server.HandleNetConn(nc)
		}
	}()

	os.Exit(m.Run())
}

func TestOpenDJ(t *testing.T) {
	var (
		playbackError error
		songStarted   = make(chan struct{}, 2)
		songEnded     = make(chan struct{}, 2)
		dj            = opendj.NewDj(nil)
	)

	dj.AddNewSongHandler(func(entry opendj.QueueEntry) {
		t.Logf("Song started: %s", entry.Media.Title)
		songStarted <- struct{}{}
	})

	dj.AddEndOfSongHandler(func(entry opendj.QueueEntry, err error) {
		t.Logf("Song ended: %s", entry.Media.Title)
		if err != nil {
			playbackError = err
		}
		songEnded <- struct{}{}
	})

	songs := []opendj.QueueEntry{
		{
			Media: opendj.Media{
				URL: "https://www.youtube.com/watch?v=jNQXAC9IVRw",
			},
			Owner: "User1",
		},
		{
			Media: opendj.Media{
				URL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			},
			Owner: "User2",
		},
	}

	for _, song := range songs {
		dj.AddEntry(song)
	}

	rtmpURL := "rtmp://localhost:1935/live/test-stream"

	go func() {
		t.Logf("Starting playback to %s", rtmpURL)
		dj.Play(t.Context(), rtmpURL)
	}()

	t.Log("Waiting for first song to start")
	select {
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for first song to start")
	case <-songStarted:
		t.Log("First song started")
	}

	t.Log("Skipping song")
	dj.Skip()

	t.Log("Waiting for second song to start")
	select {
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for second song to start")
	case <-songStarted:
		t.Log("Second song started")
	}

	t.Log("Waiting for second song to end")
	select {
	case <-songEnded:
		t.Log("Second song ended successfully")
	case <-time.After(30 * time.Second):
		t.Fatal("Timeout waiting for second song to end")
	}

	if playbackError != nil {
		t.Errorf("No playback error should occur, got: %v", playbackError)
	}
}
