package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const defaultAPI = "https://ostdb.net/api/v1"

var (
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	muted  = "\033[90m"
	reset  = "\033[0m"
)

type client struct {
	base string
	http *http.Client
}
type options struct {
	json        bool
	page, limit int
}

func main() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		cyan, green, yellow, muted, reset = "", "", "", "", ""
	}
	api := strings.TrimRight(os.Getenv("OSTDB_API_URL"), "/")
	if api == "" {
		api = defaultAPI
	}
	c := &client{base: api, http: &http.Client{Timeout: 20 * time.Second}}
	args, opt := parseOptions(os.Args[1:])
	if len(args) == 0 {
		interactive(c)
		return
	}
	if err := run(c, args, opt); err != nil {
		fmt.Fprintln(os.Stderr, "\n"+yellow+"Chyba: "+reset+err.Error())
		os.Exit(1)
	}
}

func parseOptions(args []string) ([]string, options) {
	o := options{page: 1, limit: 10}
	clean := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			o.json = true
		case "--page":
			if i+1 < len(args) {
				i++
				o.page, _ = strconv.Atoi(args[i])
			}
		case "--limit":
			if i+1 < len(args) {
				i++
				o.limit, _ = strconv.Atoi(args[i])
			}
		case "-h", "--help":
			clean = append(clean, "help")
		default:
			clean = append(clean, args[i])
		}
	}
	if o.page < 1 {
		o.page = 1
	}
	if o.limit < 1 || o.limit > 100 {
		o.limit = 10
	}
	return clean, o
}

func run(c *client, args []string, o options) error {
	command := strings.ToLower(args[0])
	value := strings.TrimSpace(strings.Join(args[1:], " "))
	switch command {
	case "help", "--help":
		printHelp()
	case "search", "find":
		if value == "" {
			return errors.New("usage: ostdb search <game name>")
		}
		return c.search(value, o)
	case "game", "detail":
		if value == "" {
			return errors.New("usage: ostdb game <slug or IGDB ID>")
		}
		return c.game(value, o)
	case "soundtrack", "ost":
		if value == "" {
			return errors.New("usage: ostdb soundtrack <ID>")
		}
		return c.get("/soundtracks/"+url.PathEscape(value), o, "soundtrack")
	case "series":
		if value == "" {
			return c.get("/series", o, "series")
		}
		return c.get("/series/"+url.PathEscape(value), o, "series")
	case "stats", "status":
		return c.get("/stats", o, "stats")
	case "updates", "recent":
		return c.get("/updates?limit="+strconv.Itoa(o.limit)+"&page="+strconv.Itoa(o.page), o, "updates")
	case "health":
		return c.get("/health", o, "health")
	default:
		return fmt.Errorf("unknown command %q — run ostdb help", command)
	}
	return nil
}

func (c *client) search(q string, o options) error {
	path := "/games?q=" + url.QueryEscape(q) + "&limit=" + strconv.Itoa(o.limit) + "&page=" + strconv.Itoa(o.page)
	return c.get(path, o, "search")
}

func (c *client) game(id string, o options) error {
	path := "/games/" + url.PathEscape(id)
	if n, err := strconv.Atoi(id); err == nil && n > 0 {
		path = "/games/igdb/" + strconv.Itoa(n)
	}
	return c.get(path, o, "game")
}

func (c *client) get(path string, o options, kind string) error {
	payload, err := c.fetch(path)
	if err != nil {
		return err
	}
	if o.json {
		pretty, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}
	printResult(kind, payload)
	return nil
}

func (c *client) fetch(path string) (any, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API is unavailable: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("API returned invalid data (%d)", res.StatusCode)
	}
	if res.StatusCode >= 400 {
		return nil, apiError(payload, res.StatusCode)
	}
	return payload, nil
}

func apiError(payload any, status int) error {
	if root, ok := payload.(map[string]any); ok {
		if e, ok := root["error"].(map[string]any); ok {
			if msg, ok := e["message"].(string); ok {
				return fmt.Errorf("%s (%d)", msg, status)
			}
		}
	}
	return fmt.Errorf("API returned HTTP %d", status)
}

func printResult(kind string, payload any) {
	root, _ := payload.(map[string]any)
	data := root["data"]
	fmt.Printf("\n%sOSTDB%s  %s\n%s\n", cyan, reset, strings.ToUpper(kind), strings.Repeat("─", 52))
	switch kind {
	case "search":
		printSearch(data, root["meta"])
	case "game":
		printGame(data)
	case "soundtrack":
		printSoundtrack(data)
	case "series":
		printSeries(data)
	case "stats":
		printStats(data)
	case "updates":
		printUpdates(data)
	case "health":
		printHealth(data)
	default:
		printJSON(data)
	}
	if meta, ok := root["meta"].(map[string]any); ok && meta["total"] != nil {
		fmt.Printf("\n%spage %v of %v%s\n", muted, meta["page"], meta["total_pages"], reset)
	}
}

func printSearch(data, meta any) {
	items, _ := data.([]any)
	if len(items) == 0 {
		fmt.Println(muted + "No games found." + reset)
		return
	}
	for _, raw := range items {
		x, _ := raw.(map[string]any)
		fmt.Printf("%s%-36s%s  %v OST  %s/game/%s%s\n", green, text(x, "name"), reset, x["album_count"], muted, text(x, "slug"), reset)
	}
	_ = meta
}
func printGame(data any) {
	x, ok := data.(map[string]any)
	if !ok {
		printJSON(data)
		return
	}
	fmt.Printf("%s%s%s  %sIGDB %v%s\n", green, text(x, "name"), reset, muted, x["igdb_id"], reset)
	for _, k := range []string{"franchise", "collection", "rating", "genres", "platforms", "developers", "publishers", "summary"} {
		if v, ok := x[k]; ok && v != nil && fmt.Sprint(v) != "<nil>" && fmt.Sprint(v) != "" {
			fmt.Printf("%s%-13s%s %v\n", muted, k, reset, v)
		}
	}
	fmt.Printf("\n%sSoundtracks (%v)%s\n", cyan, x["soundtrack_count"], reset)
	if tracks, ok := x["soundtracks"].([]any); ok {
		for _, t := range tracks {
			printSoundtrackLine(t)
		}
	}
}
func printSoundtrack(data any) { printSoundtrackLine(data) }
func printSoundtrackLine(raw any) {
	x, _ := raw.(map[string]any)
	game, _ := x["game"].(map[string]any)
	fmt.Printf("%s• %s%s — %s (%s)\n", green, text(x, "album_name"), reset, text(game, "name"), text(x, "artists"))
	if s, ok := x["streaming"].(map[string]any); ok {
		for _, k := range []string{"spotify", "apple_music", "album_link"} {
			if v, ok := s[k].(string); ok && v != "" {
				fmt.Printf("  %s%s%s\n", muted, v, reset)
			}
		}
	}
}
func printSeries(data any) {
	x, _ := data.(map[string]any)
	fmt.Printf("%s%s%s  %v games\n\n", green, text(x, "name"), reset, x["game_count"])
	if games, ok := x["games"].([]any); ok {
		for _, g := range games {
			y, _ := g.(map[string]any)
			fmt.Printf("• %-42s %v OST\n", text(y, "name"), y["album_count"])
		}
	}
}
func printStats(data any) {
	x, _ := data.(map[string]any)
	fmt.Printf("Games: %s%v%s\nSoundtracks: %s%v%s\nSeries: %s%v%s\n", green, x["games"], reset, green, x["soundtracks"], reset, green, x["series"], reset)
	if p, ok := x["platform_links"].(map[string]any); ok {
		fmt.Printf("Spotify: %v  Apple Music: %v  Other links: %v\n", p["spotify"], p["apple_music"], p["album_link"])
	}
}
func printUpdates(data any) {
	items, _ := data.([]any)
	for _, raw := range items {
		x, _ := raw.(map[string]any)
		g, _ := x["game"].(map[string]any)
		fmt.Printf("• %s — %s %s\n", text(g, "name"), text(x, "event"), text(x, "album_name"))
	}
}
func printHealth(data any) {
	x, _ := data.(map[string]any)
	fmt.Printf("%s● API %s%s  database: %v\n", green, text(x, "status"), reset, x["database"])
}
func printJSON(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func text(x map[string]any, key string) string {
	if s, ok := x[key].(string); ok && s != "" {
		return s
	}
	return "—"
}

func interactive(c *client) {
	menu := []string{"Search games", "Game details", "Soundtrack details", "Browse series", "Catalog stats", "Recent updates", "API health", "Help", "Quit"}
	if err := setRawTerminal(true); err != nil {
		interactiveLineMode(c)
		return
	}
	defer setRawTerminal(false)
	selected := 0
	for {
		clearScreen()
		printLogo()
		fmt.Printf("%s  Use ↑ ↓ to navigate · Enter to select · q to quit%s\n\n", muted, reset)
		for i, item := range menu {
			if i == selected {
				fmt.Printf("  %s› %s%-24s%s\n", cyan, reset, item, cyan)
			} else {
				fmt.Printf("    %-24s\n", item)
			}
		}
		key, err := readKey()
		if err != nil || key == "q" || key == "Q" {
			return
		}
		if key == "up" {
			selected = (selected + len(menu) - 1) % len(menu)
		}
		if key == "down" {
			selected = (selected + 1) % len(menu)
		}
		if key == "enter" {
			if menu[selected] == "Quit" {
				return
			}
			setRawTerminal(false)
			clearScreen()
			if err := menuAction(c, selected); err != nil {
				fmt.Fprintln(os.Stderr, yellow+"Error: "+reset+err.Error())
			}
			fmt.Printf("\n%sPress Enter to return to the menu...%s", muted, reset)
			bufio.NewReader(os.Stdin).ReadString('\n')
			setRawTerminal(true)
		}
	}
}

func printLogo() {
	fmt.Printf("%s  ██████  ███████ ████████ ██████  ██████%s\n", cyan, reset)
	fmt.Printf("%s ██    ██ ██         ██    ██   ██ ██   ██%s\n", cyan, reset)
	fmt.Printf("%s ██    ██ ███████    ██    ██   ██ ██████%s\n", cyan, reset)
	fmt.Printf("%s ██    ██      ██    ██    ██   ██ ██   ██%s\n", cyan, reset)
	fmt.Printf("%s  ██████  ███████    ██    ██████  ██   ██%s\n", cyan, reset)
	fmt.Printf("\n  %sVideo game soundtrack database%s\n\n", muted, reset)
}

func clearScreen() { fmt.Print("\033[H\033[2J") }

func menuAction(c *client, selected int) error {
	reader := bufio.NewReader(os.Stdin)
	prompt := func(label string) (string, error) {
		fmt.Printf("%s%s%s ", cyan, label, reset)
		value, err := reader.ReadString('\n')
		return strings.TrimSpace(value), err
	}
	switch selected {
	case 0:
		value, err := prompt("Search games: ")
		if err != nil || value == "" {
			return errors.New("a game name is required")
		}
		return interactiveSearch(c, value)
	case 1:
		value, err := prompt("Game slug or IGDB ID: ")
		if err != nil || value == "" {
			return errors.New("a game identifier is required")
		}
		return interactiveGame(c, value)
	case 2:
		value, err := prompt("Soundtrack ID: ")
		if err != nil || value == "" {
			return errors.New("a soundtrack ID is required")
		}
		return c.get("/soundtracks/"+url.PathEscape(value), options{page: 1, limit: 10}, "soundtrack")
	case 3:
		value, err := prompt("Series slug (leave empty for all): ")
		if err != nil {
			return err
		}
		if value == "" {
			return interactiveSeriesList(c)
		}
		payload, err := c.fetch("/series/" + url.PathEscape(value))
		if err != nil {
			return err
		}
		printResult("series", payload)
		return interactiveSeriesGames(c, payload)
	case 4:
		return c.get("/stats", options{page: 1, limit: 10}, "stats")
	case 5:
		return c.get("/updates?limit=10&page=1", options{page: 1, limit: 10}, "updates")
	case 6:
		return c.get("/health", options{page: 1, limit: 10}, "health")
	case 7:
		printHelp()
		return nil
	}
	return nil
}

func interactiveSearch(c *client, query string) error {
	payload, err := c.fetch("/games?q=" + url.QueryEscape(query) + "&limit=25&page=1")
	if err != nil {
		return err
	}
	root, _ := payload.(map[string]any)
	items, _ := root["data"].([]any)
	if len(items) == 0 {
		printResult("search", payload)
		return nil
	}
	index, ok := pickItems("Search results", items, func(item any) string {
		x, _ := item.(map[string]any)
		return fmt.Sprintf("%s  ·  %v soundtracks", text(x, "name"), x["album_count"])
	})
	if !ok {
		return nil
	}
	game, _ := items[index].(map[string]any)
	return interactiveGame(c, text(game, "slug"))
}

func interactiveGame(c *client, identifier string) error {
	path := "/games/" + url.PathEscape(identifier)
	if id, err := strconv.Atoi(identifier); err == nil && id > 0 {
		path = "/games/igdb/" + strconv.Itoa(id)
	}
	payload, err := c.fetch(path)
	if err != nil {
		return err
	}
	printResult("game", payload)
	root, _ := payload.(map[string]any)
	data, _ := root["data"].(map[string]any)
	tracks, _ := data["soundtracks"].([]any)
	if len(tracks) == 0 {
		return nil
	}
	index, ok := pickItems("Choose a soundtrack", tracks, func(item any) string {
		x, _ := item.(map[string]any)
		return fmt.Sprintf("%s  ·  %s", text(x, "album_name"), text(x, "artists"))
	})
	if ok {
		clearScreen()
		printLogo()
		fmt.Println("Soundtrack details")
		printSoundtrack(tracks[index])
	}
	return nil
}

func interactiveSeriesList(c *client) error {
	payload, err := c.fetch("/series?limit=25&page=1")
	if err != nil {
		return err
	}
	root, _ := payload.(map[string]any)
	items, _ := root["data"].([]any)
	if len(items) == 0 {
		printResult("series", payload)
		return nil
	}
	index, ok := pickItems("Browse series", items, func(item any) string {
		x, _ := item.(map[string]any)
		return fmt.Sprintf("%s  ·  %v games", text(x, "name"), x["game_count"])
	})
	if !ok {
		return nil
	}
	series, _ := items[index].(map[string]any)
	detail, err := c.fetch("/series/" + url.PathEscape(text(series, "slug")))
	if err != nil {
		return err
	}
	printResult("series", detail)
	return interactiveSeriesGames(c, detail)
}

func interactiveSeriesGames(c *client, payload any) error {
	root, _ := payload.(map[string]any)
	data, _ := root["data"].(map[string]any)
	games, _ := data["games"].([]any)
	if len(games) == 0 {
		return nil
	}
	index, ok := pickItems("Choose a game", games, func(item any) string {
		x, _ := item.(map[string]any)
		return fmt.Sprintf("%s  ·  %v soundtracks", text(x, "name"), x["album_count"])
	})
	if ok {
		game, _ := games[index].(map[string]any)
		return interactiveGame(c, text(game, "slug"))
	}
	return nil
}

func pickItems(title string, items []any, label func(any) string) (int, bool) {
	if err := setRawTerminal(true); err != nil {
		return -1, false
	}
	defer setRawTerminal(false)
	selected := 0
	for {
		clearScreen()
		printLogo()
		fmt.Printf("%s%s%s\n%s  Use ↑ ↓ to navigate · Enter to open · q to go back%s\n\n", cyan, title, reset, muted, reset)
		for i, item := range items {
			if i == selected {
				fmt.Printf("  %s› %s%s%s\n", cyan, reset, label(item), reset)
			} else {
				fmt.Printf("    %s\n", label(item))
			}
		}
		key, err := readKey()
		if err != nil || key == "q" || key == "Q" {
			return -1, false
		}
		if key == "up" {
			selected = (selected + len(items) - 1) % len(items)
		}
		if key == "down" {
			selected = (selected + 1) % len(items)
		}
		if key == "enter" {
			return selected, true
		}
	}
}

func interactiveLineMode(c *client) {
	fmt.Println("OSTDB — interactive mode (arrow keys unavailable in this terminal)")
	fmt.Println("Type a command, or 'quit' to exit.")
	s := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf("\nostdb> ")
		if !s.Scan() {
			return
		}
		line := strings.TrimSpace(s.Text())
		if line == "quit" || line == "exit" {
			return
		}
		if line != "" {
			args, o := parseOptions(strings.Fields(line))
			if err := run(c, args, o); err != nil {
				fmt.Fprintln(os.Stderr, yellow+"Error: "+reset+err.Error())
			}
		}
	}
}

func setRawTerminal(raw bool) error {
	args := []string{"sane"}
	if raw {
		args = []string{"-icanon", "-echo", "min", "1", "time", "0"}
	}
	command := exec.Command("stty", args...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}

func readKey() (string, error) {
	var b [1]byte
	if _, err := os.Stdin.Read(b[:]); err != nil {
		return "", err
	}
	switch b[0] {
	case 13, 10:
		return "enter", nil
	case 'q', 'Q':
		return string(b[0]), nil
	case 27:
		var seq [2]byte
		if _, err := io.ReadFull(os.Stdin, seq[:]); err != nil {
			return "", err
		}
		if seq[0] == '[' && seq[1] == 'A' {
			return "up", nil
		}
		if seq[0] == '[' && seq[1] == 'B' {
			return "down", nil
		}
	}
	return "", nil
}

func printHelp() {
	fmt.Printf("%sOSTDB CLI%s — video game soundtrack database\n\nUsage:\n  ostdb                         interactive mode\n  ostdb search <name>           search games\n  ostdb game <slug or ID>       game details and soundtracks\n  ostdb soundtrack <ID>         soundtrack details\n  ostdb series [slug]           series or series list\n  ostdb stats                   catalog statistics\n  ostdb updates                 recent changes\n  ostdb health                  API health\n\nOptions:\n  --json                        print raw JSON\n  --limit N                     results per page (1–100)\n  --page N                      result page\n\nThe API can be changed with OSTDB_API_URL.\n", cyan, reset)
}
