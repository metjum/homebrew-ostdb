# OSTDB CLI for macOS

`ostdb` is a terminal client for [OSTDB](https://ostdb.net), the video game soundtrack database. It uses the public read-only API and does not require Spotify, GitHub, or any token.

## Installation

```sh
brew tap metjum/ostdb
brew install metjum/ostdb/ostdb
```

Homebrew 6 requires explicit trust for third-party taps. To use the short command `brew install ostdb`, trust only this formula first:

```sh
brew tap metjum/ostdb
brew trust --formula metjum/ostdb/ostdb
brew install ostdb
```

To install the latest development version directly from GitHub, use `brew install --HEAD ostdb`.

After installation:

```sh
ostdb
```

Running `ostdb` without a command opens interactive mode. Commands can also be used directly:

```sh
ostdb search "The Witcher"
ostdb game the-witcher-3-wild-hunt
ostdb game 348330
ostdb soundtrack SOUNDTRACK_ID
ostdb stats
ostdb updates --limit 20
ostdb search "Forza" --json
```

## Local development

```sh
cd ostdb
go run . search "Forza"
go test ./...
```

The default API is `https://ostdb.net/api/v1`. To use a local or test API:

```sh
OSTDB_API_URL=http://localhost:8787/api/v1 ostdb health
```

The formula contains the CLI source directly in this tap repository. After publishing a versioned release, the URL in `Formula/ostdb.rb` should be changed to the tagged tarball and given its SHA-256 checksum.
