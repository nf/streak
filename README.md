#Streak
Streak is a command-line productivity tool based around the "Seinfeld method"

> [Seinfeld](https://en.wikipedia.org/wiki/Jerry_Seinfeld) revealed a unique calendar system he uses to pressure
himself to write. Here's how it works.

> He told me to get a big wall calendar that has a whole year on one page
and hang it on a prominent wall. The next step was to get a big red
magic marker.

> He said for each day that I do my task of writing, I get to put a big
red X over that day. "After a few days you'll have a chain. Just keep
at it and the chain will grow longer every day. You'll like seeing that
chain, especially when you get a few weeks under your belt. Your only
job next is to not break the chain."

> "Don't break the chain," he said again for emphasis.

> **Source:** http://lifehacker.com/281626/jerry-seinfelds-productivity-secret

Streak uses the Google Calendar API to maintain a calendar named "Streaks"
(that you must create yourself). The calendar consists of multi-day entries
titled "Streak" that are created, extended, or shortened by this tool.  The
idea is that you run this tool every day whenever you've done the thing that
you're trying to push yourself to do. Then whenever you look at your Google
Calendar you'll see your streaks and feel proud/ashamed of yourself.

##Usage
	Usage: streak [OPTION...]
	  -cachefile="/home/erb/.streak-request-token": Authentication token cache file
	  -cal="Streaks": Streak calendar name
	  -create=false: Create calendar if missing
	  -event="Streak": Streak event name
	  -offset=0: Day offset
	  -remove: Remove day from streak

###Examples:

	streak
		Add today to a streak (or create a streak if none exists).
	streak -create
		Add today to a streak using default calendar "Streaks", creates the calendar first if missing.
	streak -remove
		Remove today from a streak.
	streak -offset -1
		Add yesterday to a streak (or create if none exists).
	streak -offset -1 -remove
		Remove yesterday from a streak
	streak -cal="Streaks Test" -create
		Add today to a streak in calendar "Streaks Example", creates the calendar first if missing.

-----
Andrew Gerrand <adg@golang.org>
