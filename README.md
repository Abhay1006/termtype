# termtype

A minimal, fast typing test for the terminal, written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

It has timed, word-count and quote modes, live WPM, a per-second WPM graph,
personal bests saved locally, and colour themes, all in a single binary
with no runtime dependencies.

<img width="1080" height="644" alt="termtype demo" src="https://github.com/user-attachments/assets/52ca1488-a477-41c4-9bde-8661f7a69bb4" />

## Features

- **Three test modes**
  - **time**: type for 15, 30 or 60 seconds (or any length via `-t`). The text never runs out.
  - **words**: type 10, 25 or 50 words (or any count via `-w`).
  - **quote**: type one of the bundled quotes from literature.
- **Punctuation and numbers**: optional capitalised sentences with `. , ? ! ; :` and random numbers.
- **Live feedback**: correct characters are coloured, mistakes are underlined, a wrongly typed space is highlighted, and live WPM plus time or word progress are shown while you type.
- **Detailed results**: net WPM, raw WPM, accuracy, consistency, correct/incorrect characters, elapsed time, and a sparkline of your WPM for each second with its peak.
- **Personal bests**: every result is saved. termtype shows your previous best and your average over the last 10 tests for the same settings, and tells you when you set a new best.
- **Smooth text view**: words wrap to the terminal width (up to 60 columns) and a three-line window scrolls as you type, so the next line is always visible.
- **Word deletion**: Ctrl+Backspace, Ctrl+W or Alt+Backspace delete a whole word.
- **Themes**: five built-in colour themes, plus custom colours through a config file.
- **Safe storage**: history is written atomically (temp file + rename), so a crash can't corrupt it.

## Installation

You need [Go](https://go.dev/dl/) 1.27 or newer.

```sh
git clone https://github.com/Abhay1006/termtype.git
cd termtype
go build -o termtype .
./termtype
```

To make it available everywhere, move the binary onto your `PATH`, for example:

```sh
mv termtype ~/.local/bin/
```

## Usage

```sh
termtype              # 30 second time test
termtype -t 60        # 60 second time test
termtype -w 50        # 50 word test
termtype -q           # quote test
termtype -p -n        # add punctuation and numbers
termtype -theme ocean # use a different colour theme
```

### Flags

| Flag          | Description                                         | Default   |
| ------------- | --------------------------------------------------- | --------- |
| `-t <sec>`    | Test length in seconds (time mode)                  | `30`      |
| `-w <n>`      | Number of words; switches to words mode             | off       |
| `-q`          | Type a quote instead of random words                | off       |
| `-p`          | Add punctuation and capital letters                 | off       |
| `-n`          | Add numbers                                         | off       |
| `-theme <name>` | Colour theme: `default`, `forest`, `mono`, `ocean`, `warm` | `default` |

`-q` and `-w` can't be used together.

### Controls

The test starts on your first keystroke. Settings can be changed before you start typing.

| Key                                   | Action                                   |
| ------------------------------------- | ---------------------------------------- |
| `↑` / `↓`                             | Switch mode (time → words → quote)       |
| `←` / `→`                             | Change length (or pick a new quote)      |
| `Ctrl+P`                              | Toggle punctuation                       |
| `Ctrl+N`                              | Toggle numbers                           |
| `Backspace`                           | Delete a character                       |
| `Ctrl+Backspace` / `Ctrl+W` / `Alt+Backspace` | Delete a word                    |
| `Tab`                                 | Restart with a new text                  |
| `Esc` / `Ctrl+C`                      | Quit                                     |

## How scores are calculated

A "word" is five characters, the usual convention for typing tests.

- **WPM** (net) = correct characters ÷ 5 ÷ minutes elapsed
- **Raw WPM** = all typed characters ÷ 5 ÷ minutes elapsed
- **Accuracy** = correct characters ÷ typed characters × 100
- **Consistency** = 100 × (1 − stddev(per-second WPM) ÷ mean(per-second WPM)), clamped to 0–100
- **WPM graph** = characters typed during each second × 60 ÷ 5

Personal bests and averages compare only tests with the same mode, length,
punctuation and numbers settings.

## Configuration

termtype stores its files in your user config directory:

| OS      | Location                                      |
| ------- | --------------------------------------------- |
| Linux   | `~/.config/termtype/`                         |
| macOS   | `~/Library/Application Support/termtype/`     |
| Windows | `%AppData%\termtype\`                         |

### `config.json` (optional)

Every field is optional. Colours can be ANSI colour numbers (`"10"`) or hex
values (`"#a3be8c"`), and any colour you set overrides the chosen theme.

```json
{
  "theme": "ocean",
  "colors": {
    "correct": "#7fdbca",
    "wrong": "#ff5874",
    "pending": "#5f7e97",
    "accent": "#ffcc00"
  }
}
```

The `-theme` flag overrides the theme set in the config file.

### `history.json`

Every finished test is appended here, for example:

```json
{
  "date": "2026-11-17T10:30:00Z",
  "mode": "time",
  "length": 30,
  "punctuation": true,
  "wpm": 82.4,
  "raw": 88.0,
  "accuracy": 96.3,
  "consistency": 84.5
}
```

Delete this file to reset your personal bests.

## Project structure

```
.
├── main.go      # Bubble Tea model: input handling, test logic, rendering, flags
├── text.go      # Random word, punctuation, number and quote generation
├── history.go   # Saving/loading results, personal best and average summary
├── theme.go     # Built-in themes, config.json loading, Lip Gloss styles
├── words.txt    # 642 common English words (embedded in the binary)
└── quotes.txt   # Quotes for quote mode (embedded in the binary)
```

`words.txt` and `quotes.txt` are embedded with `go:embed`, so the built
binary works on its own. To add quotes, put one per line in `quotes.txt` and
rebuild.

## Built with

- [Go](https://go.dev/)
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): terminal UI framework (Elm architecture)
- [Lip Gloss](https://github.com/charmbracelet/lipgloss): terminal styling and layout

## License

[MIT](LICENSE)
