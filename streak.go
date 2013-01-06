// Copyright 2011 The goauth2 Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This program makes a call to the specified API, authenticated with OAuth2.
// a list of example APIs can be found at https://code.google.com/oauthplayground/
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
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
		ClientId:     "120233572441-d8vmojicfgje467joivr5a7j52dg2gnc.apps.googleusercontent.com",
		ClientSecret: "6vu85BLgDWH49y5vGCDdgPWL",
		Scope:        "https://www.googleapis.com/auth/calendar",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
		RedirectURL:  "",
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

func addToStreak(calId string, today time.Time) (bool, error) {
	events, err := service.Events.List(calId).Do()
	if err != nil {
		return false, err
	}
	items := events.Items
	sort.Sort(eventsByStartDate(items))

	for _, e := range items {
		if e.Start.Date == "" || e.End.Date == "" {
			// Skip non-all-day event.
			continue
		}
		start, end := parseDate(e.Start.Date), parseDate(e.End.Date)
		if start.After(today) {
			if start.Add(-day).Equal(today) {
				// This event starts tomorrow, update it to start today.
				e.Start.Date = today.Format(dateFormat)
				_, err = service.Events.Update(calId, e.Id, e).Do()
				return true, err
			}
			// This event is too far in the future.
			continue
		}
		if end.After(today) {
			// Today fits inside this event, nothing to do.
			return false, nil
		}
		if end.Equal(today) {
			// This event ends today, update it to end tomorrow.
			e.End.Date = today.Add(day).Format(dateFormat)
			_, err = service.Events.Update(calId, e.Id, e).Do()
			return true, err
		}
	}

	// No existing events cover or are adjacent to today, so create one.
	return true, createEvent(calId, today, today.Add(day))
}

func removeFromStreak(calId string, today time.Time) (bool, error) {
	events, err := service.Events.List(calId).Do()
	if err != nil {
		return false, err
	}

	for _, e := range events.Items {
		if e.Start.Date == "" || e.End.Date == "" {
			// Skip non-all-day event.
			continue
		}
		start, end := parseDate(e.Start.Date), parseDate(e.End.Date)
		if start.After(today) || end.Before(today) || end.Equal(today) {
			// This event is too far in the future or past.
			continue
		}
		if start.Equal(today) {
			if end.Equal(today.Add(day)) {
				// Remove event.
				return true, service.Events.Delete(calId, e.Id).Do()
			}
			// Shorten to begin tomorrow.
			e.Start.Date = start.Add(day).Format(dateFormat)
			_, err = service.Events.Update(calId, e.Id, e).Do()
			return true, err
		}
		if end.Equal(today.Add(day)) {
			// Shorten to end today.
			e.End.Date = today.Format(dateFormat)
			_, err = service.Events.Update(calId, e.Id, e).Do()
			return true, err
		}

		// Split into two events.
		// Shorten first event to end today.
		e.End.Date = today.Format(dateFormat)
		_, err = service.Events.Update(calId, e.Id, e).Do()
		if err != nil {
			return true, err
		}
		// Create second event that starts tomorrow.
		return true, createEvent(calId, today.Add(day), end)
	}
	return false, nil
}

func mergeAdjacentEvents(calId string) error {
	events, err := service.Events.List(calId).Do()
	if err != nil {
		return err
	}
	items := events.Items
	sort.Sort(eventsByStartDate(items))

	var prevEnd time.Time
	for i, e := range items {
		if e.Start.Date == "" || e.End.Date == "" {
			// Skip non-all-day event.
			continue
		}
		start, end := parseDate(e.Start.Date), parseDate(e.End.Date)
		if start.Equal(prevEnd) {
			// Merge events.
			// Extend this event to begin where the previous one did.
			prev := items[i-1]
			e.Start = prev.Start
			_, err = service.Events.Update(calId, e.Id, e).Do()
			if err != nil {
				return err
			}
			// Delete the previous event.
			err = service.Events.Delete(calId, prev.Id).Do()
			if err != nil {
				return err
			}
		}
		prevEnd = end
	}
	return nil
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

type eventsByStartDate []*calendar.Event

func (s eventsByStartDate) Len() int           { return len(s) }
func (s eventsByStartDate) Less(i, j int) bool { return s[i].Start.Date < s[j].Start.Date }
func (s eventsByStartDate) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
