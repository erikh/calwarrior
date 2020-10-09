# calwarrior: make taskwarrior work with google calendar

I use both services extensively and would like one place to work. I prefer
taskwarrior but get notifications, etc on my phone from google calendar. In
summary, google calendar makes me more productive, but I hate it.

I found a project that didn't work but seeded the idea for this program.

## Installation:

Install [golang](https://golang.org).

```bash
go get github.com/erikh/calwarrior
```

## Usage:

```bash
$ calwarrior # launches with defaults
$ calwarrior --help # it has options and even help!
```

## Google Calendar OAuth2 credentials

`calwarrior` needs oauth2 credentials to talk to google calendar.

You can accomplish this one of 3 ways :

- The first two require you generate your own oauth2 client; this is **strongly recommended**.
  - Setting the environment variable `CALWARRIOR_CREDENTIALS` to the `credentials.json` file.
  - Putting the `credentials.json` in the `calwarrior` settings directory.
- Finally, you can try it by using the default oauth2 credentials embedded in the source code.

## Configuration Directory

`calwarrior` will attempt to launch your browser and stuff credentials in your
home directory (`$HOME/Library/calwarrior` or `$HOME/.config/calwarrior` on
Linux). Follow the instructions and paste in the code into the terminal to save
the token. It works with the default `task` or `taskw` command on your `$PATH`.

## Troubleshooting

If you start seeing error messages like this:

```
Error modifying calendar event: googleapi: Error 400: Invalid time zone definition for start time.
```

This is because your `TZ` or `ZONEINFO` environment variables are not set. `man tzname` for more information.

## Bugs

It's not very well tested at all, and the code is pretty ugly. But it seems to
work for me.

## Author

Erik Hollensbe <erik+github@hollensbe.org>
