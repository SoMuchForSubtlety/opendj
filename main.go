package opendj

import (
	"errors"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/googleapi/transport"
	"google.golang.org/api/youtube/v3"
)

// Dj ...
type Dj struct {
	ytServ *youtube.Service

	waitingQueue Queue
	currentEntry QueueEntry

	runningCommand *exec.Cmd

	playbackFeed   chan QueueEntry
	playbackErrors chan error

	songStarted time.Time
}

// Video ...
type Video struct {
	Title    string
	ID       string
	Duration time.Duration
}

// QueueEntry ...
type QueueEntry struct {
	Video      Video
	User       string
	Dedication string
}

// Queue ...
type Queue struct {
	Items []QueueEntry
	sync.Mutex
}

const youtubeURLStart = "https://www.youtube.com/watch?v="

// NewDj returns ...
func NewDj(youtubeKey string, queue []QueueEntry) (dj *Dj, playbackFeed <-chan QueueEntry, playbackErrorFeed <-chan error, err error) {
	dj = &Dj{}
	dj.waitingQueue.Items = queue
	dj.playbackFeed = make(chan QueueEntry, 100)
	dj.playbackErrors = make(chan error, 100)

	client := &http.Client{Transport: &transport.APIKey{Key: youtubeKey}}
	ytServ, err := youtube.New(client)
	if err != nil {
		return nil, nil, nil, err
	}
	dj.ytServ = ytServ

	return dj, dj.playbackFeed, dj.playbackErrors, nil
}

// GetYTVideo ...
func (dj *Dj) GetYTVideo(videoID string) (video Video, err error) {
	res, err := dj.ytServ.Videos.List("id,snippet,contentDetails").Id(videoID).Do()
	if err != nil {
		return video, err
	} else if len(res.Items) < 1 {
		return video, errors.New("Video not found")
	}
	video.Title = res.Items[0].Snippet.Title
	video.ID = res.Items[0].Id
	video.Duration, _ = time.ParseDuration(strings.ToLower(res.Items[0].ContentDetails.Duration[2:]))
	return video, nil
}

// AddEntry ...
func (dj *Dj) AddEntry(newEntry QueueEntry) {
	dj.waitingQueue.Lock()
	dj.waitingQueue.Items = append(dj.waitingQueue.Items, newEntry)
	dj.waitingQueue.Unlock()
}

// InsertEntry ...
func (dj *Dj) InsertEntry(newEntry QueueEntry, index int) error {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	if index < 0 {
		return errors.New("index out of range")
	} else if index >= len(dj.waitingQueue.Items) {
		dj.waitingQueue.Items = append(dj.waitingQueue.Items, newEntry)
		return nil
	}
	dj.waitingQueue.Items = append(dj.waitingQueue.Items, QueueEntry{})
	copy(dj.waitingQueue.Items[index+1:], dj.waitingQueue.Items[index:])
	dj.waitingQueue.Items[index] = newEntry
	return nil
}

// RemoveIndex ...
func (dj *Dj) RemoveIndex(index int) error {
	dj.waitingQueue.Lock()
	if index >= len(dj.waitingQueue.Items) || index < 0 {
		return errors.New("index out of range")
	}
	dj.waitingQueue.Items = append(dj.waitingQueue.Items[:index], dj.waitingQueue.Items[index+1:]...)
	dj.waitingQueue.Unlock()
	return nil
}

func (dj *Dj) pop() (QueueEntry, error) {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	if len(dj.waitingQueue.Items) < 1 {
		return QueueEntry{}, errors.New("can't pop from empty queue")
	}

	entry := dj.waitingQueue.Items[0]
	dj.waitingQueue.Items = dj.waitingQueue.Items[1:]
	return entry, nil
}

// EntryAtIndex ...
func (dj *Dj) EntryAtIndex(index int) (QueueEntry, error) {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	if index >= len(dj.waitingQueue.Items) || index < 0 {
		return QueueEntry{}, errors.New("index out of range")
	}

	entry := dj.waitingQueue.Items[index]

	return entry, nil
}

// Play ...
func (dj *Dj) Play(rtmpServer string) {
	for {
		entry, err := dj.pop()
		if err != nil {
			// TODO: not ideal, maybe have a backup playlist
			time.Sleep(time.Second * 5)
			continue
		}
		dj.currentEntry = entry

		command := exec.Command("youtube-dl", "-f", "bestaudio", "-g", youtubeURLStart+dj.currentEntry.Video.ID)
		url, err := command.Output()
		if err != nil {
			dj.playbackErrors <- err
			continue
		}

		urlProper := strings.TrimSpace(string(url))
		dj.songStarted = time.Now()

		command = exec.Command("ffmpeg", "-reconnect", "1", "-reconnect_at_eof", "1", "-reconnect_delay_max", "3", "-re", "-i", urlProper, "-codec:a", "aac", "-f", "flv", rtmpServer)
		err = command.Start()
		if err != nil {
			dj.playbackErrors <- err
			continue
		}
		dj.playbackFeed <- dj.currentEntry

		dj.runningCommand = command

		err = command.Wait()
		if err != nil {
			dj.playbackErrors <- err
		}

		dj.runningCommand = nil
	}
}

// UserPosition ...
func (dj *Dj) UserPosition(nick string) (positions []int) {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	for i, content := range dj.waitingQueue.Items {
		if content.User == nick {
			positions = append(positions, i)
		}
	}
	return positions
}

// DurationUntilUser ...
func (dj *Dj) DurationUntilUser(nick string) (durations []time.Duration) {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	dur := dj.currentEntry.Video.Duration - time.Since(dj.songStarted)
	for _, content := range dj.waitingQueue.Items {
		if content.User == nick {
			durations = append(durations, dur)
		}
		dur += content.Video.Duration
	}
	return durations
}

// AddDedication ...
func (dj *Dj) AddDedication(index int, target string) error {
	dj.waitingQueue.Lock()
	defer dj.waitingQueue.Unlock()

	if index < 0 || index >= len(dj.waitingQueue.Items) {
		return errors.New("index out of range")
	}

	dj.waitingQueue.Items[index].Dedication = target
	return nil
}
