package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"code.google.com/p/goauth2/oauth"
	"code.google.com/p/google-api-go-client/calendar/v3"
)

const (
	dateFormat = "2006-01-02"
	day        = time.Second * 60 * 60 * 24
	calSummary = "Streaks"
	evtSummary = "Streak"
)

var (
	defaultCacheFile = filepath.Join(os.Getenv("HOME"), ".streak-request-token")
	cachefile        = flag.String("cachefile", defaultCacheFile, "Authentication token cache file")
	code             = flag.String("code", "", "OAuth Authorization Code")
	offset           = flag.Int("offset", 0, "Day offset")
	remove           = flag.Bool("remove", false, "Remove day from streak")

	service *calendar.Service
)

type Streak struct {
	Start string
	End   string
}

func main() {
	flag.Parse()

	config := &oauth.Config{
		ClientId:     os.Getenv("STREAK_CLIENT_ID"),
		ClientSecret: os.Getenv("STREAK_CLIENT_SECRET"),
		Scope:        "https://www.googleapis.com/auth/calendar",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}

	transport := &oauth.Transport{Config: config}
	tokenCache := oauth.CacheFile(*cachefile)

	token, err := tokenCache.Token()
	if err != nil {
		log.Println("Cache read:", err)
		if *code == "" {
			url := config.AuthCodeURL("")
			fmt.Println("Visit this URL to get a code, then run again with -code=YOUR_CODE\n")
			fmt.Println(url)
			return
		}
		token, err = transport.Exchange(*code)
		if err != nil {
			log.Fatal("Exchange:", err)
		}
		err = tokenCache.PutToken(token)
		if err != nil {
			log.Println("Cache write:", err)
		}
	}
	transport.Token = token

	service, err = calendar.New(transport.Client())
	if err != nil {
		log.Fatal(err)
	}

	calId, err := streakCalendarId()
	if err != nil {
		log.Fatal(err)
	}

	today := parseDate(time.Now().Add(time.Duration(*offset) * day).Format(dateFormat))
	var updated bool
	if *remove {
		updated, err = removeFromStreak(calId, today)
	} else {
		updated, err = addToStreak(calId, today)
		if updated && err == nil {
			err = mergeAdjacentEvents(calId)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

func addToStreak(calId string, today time.Time) (mutated bool, err error) {
	create := true
	err = iterateEvents(calId, func(e *calendar.Event, start, end time.Time) error {
		if start.After(today) {
			if start.Add(-day).Equal(today) {
				// This event starts tomorrow, update it to start today.
				mutated = true
				e.Start.Date = today.Format(dateFormat)
				_, err = service.Events.Update(calId, e.Id, e).Do()
				return err
			}
			// This event is too far in the future.
			return Continue
		}
		if end.After(today) {
			// Today fits inside this event, nothing to do.
			create = false
			return nil
		}
		if end.Equal(today) {
			// This event ends today, update it to end tomorrow.
			mutated = true
			e.End.Date = today.Add(day).Format(dateFormat)
			_, err = service.Events.Update(calId, e.Id, e).Do()
			return err
		}
		return Continue
	})
	if err != nil {
		return
	}
	if !mutated && create {
		// No existing events cover or are adjacent to today, so create one.
		mutated = true
		err = createEvent(calId, today, today.Add(day))
	}
	return
}

func removeFromStreak(calId string, today time.Time) (mutated bool, err error) {
	err = iterateEvents(calId, func(e *calendar.Event, start, end time.Time) error {
		if start.After(today) || end.Before(today) || end.Equal(today) {
			// This event is too far in the future or past.
			return Continue
		}
		mutated = true
		if start.Equal(today) {
			if end.Equal(today.Add(day)) {
				// Remove event.
				return service.Events.Delete(calId, e.Id).Do()
			}
			// Shorten to begin tomorrow.
			e.Start.Date = start.Add(day).Format(dateFormat)
			_, err := service.Events.Update(calId, e.Id, e).Do()
			return err
		}
		if end.Equal(today.Add(day)) {
			// Shorten to end today.
			e.End.Date = today.Format(dateFormat)
			_, err := service.Events.Update(calId, e.Id, e).Do()
			return err
		}

		// Split into two events.
		// Shorten first event to end today.
		e.End.Date = today.Format(dateFormat)
		_, err = service.Events.Update(calId, e.Id, e).Do()
		if err != nil {
			return err
		}
		// Create second event that starts tomorrow.
		return createEvent(calId, today.Add(day), end)
	})
	return
}

func mergeAdjacentEvents(calId string) error {
	var prev *calendar.Event
	var prevEnd time.Time
	return iterateEvents(calId, func(e *calendar.Event, start, end time.Time) error {
		if start.Equal(prevEnd) {
			// Merge events.
			// Extend this event to begin where the previous one did.
			e.Start = prev.Start
			_, err := service.Events.Update(calId, e.Id, e).Do()
			if err != nil {
				return err
			}
			// Delete the previous event.
			err = service.Events.Delete(calId, prev.Id).Do()
			if err != nil {
				return err
			}
		}
		prev = e
		prevEnd = end
		return Continue
	})
}

var Continue = errors.New("continue")

type iteratorFunc func(e *calendar.Event, start, end time.Time) error

func iterateEvents(calId string, fn iteratorFunc) error {
	var nextPageToken string
	for {
		call := service.Events.List(calId)
		call.SingleEvents(true)
		call.OrderBy("startTime")
		if nextPageToken != "" {
			call.PageToken(nextPageToken)
		}
		events, err := call.Do()
		if err != nil {
			return err
		}
		for _, e := range events.Items {
			if e.Start.Date == "" || e.End.Date == "" || e.Summary != evtSummary {
				// Skip non-all-day event or non-streak events.
				continue
			}
			start, end := parseDate(e.Start.Date), parseDate(e.End.Date)
			if err := fn(e, start, end); err != Continue {
				return err
			}
		}
		nextPageToken = events.NextPageToken
		if nextPageToken == "" {
			return nil
		}
	}
	panic("unreachable")
}

func createEvent(calId string, start, end time.Time) error {
	e := &calendar.Event{
		Summary: evtSummary,
		Start:   &calendar.EventDateTime{Date: start.Format(dateFormat)},
		End:     &calendar.EventDateTime{Date: end.Format(dateFormat)},
	}
	_, err := service.Events.Insert(calId, e).Do()
	return err
}

func parseDate(s string) time.Time {
	t, err := time.Parse(dateFormat, s)
	if err != nil {
		panic(err)
	}
	return t
}

func streakCalendarId() (string, error) {
	list, err := service.CalendarList.List().Do()
	if err != nil {
		return "", err
	}
	for _, entry := range list.Items {
		if entry.Summary == calSummary {
			return entry.Id, nil
		}
	}
	return "", errors.New("couldn't find calendar named 'Streaks'")
}
