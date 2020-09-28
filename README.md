# calwarrior: make taskwarrior work with google calendar

I use both services extensively and would like one place to work. I prefer
taskwarrior but get notifications, etc on my phone from google calendar. In
summary, google calendar makes me more productive, but I hate it.

I found a project that didn't work but seeded the idea for this program.

## Installation:

This will move to github eventually, so these URLs are **not** permanent.

Install [golang](https://golang.org).

```bash
go get github.com/erikh/calwarrior
```

## Usage:

```bash
$ calwarrior # launches with defaults
$ calwarrior --help # it has options and even help!
```

`calwarrior` will attempt to launch your browser and stuff credentials in your
home directory (`$HOME/Library/calwarrior` or `$HOME/.config/calwarrior` on
Linux). Follow the instructions and paste in the code into the terminal to save
the token. It works with the default `task` or `taskw` command on your `$PATH`.

`calwarrior` also has a hard-coded authentication credential/application for
OAuth, but you can create your own on your google sign-in services page, and
set `CALWARRIOR_CREDENTIALS` in the environment to the OAuth application's
`credentials.json`. This will override the hard-coded defaults if you don't
feel comfortable using another client credential.

## Bugs

It's not very well tested at all, and the code is pretty ugly. But it seems to
work for me.

## Author

Erik Hollensbe <erik+github@hollensbe.org>
