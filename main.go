package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Action func() error

type Controller struct {
	mu sync.Mutex

	altMode    int
	artistIndex int
	artistList  []string

	modesMap []map[uint16]Action
	baseDir  string
}

func main() {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to resolve executable path:", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(exe)

	c := &Controller{
		altMode:     0,
		artistIndex: -1,
		baseDir:     baseDir,
		modesMap:    []map[uint16]Action{{}, {}, {}},
	}

	c.setupMappings()

	if err := c.listen("/dev/input/event0"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ============================================================
// Linux input definitions
// ============================================================

type inputEvent struct {
	Time  syscallTimeval
	Type  uint16
	Code  uint16
	Value int32
}

type syscallTimeval struct {
	Sec  int64
	Usec int64
}

// Linux input event types
const (
	EV_KEY = 0x01
)

// Controller key codes
const (
	KEY_WAKEUP      = 143
	KEY_VIDEO_NEXT  = 0x00f1
	KEY_PLAY        = 207
	KEY_PAUSE       = 119
	KEY_MUTE        = 0x0071
	KEY_INFO        = 0x0166
	KEY_STOP        = 128
	KEY_EJECT       = 161
	KEY_VOLUMEDOWN  = 114
	KEY_VOLUMEUP    = 115
	KEY_NEXTSONG    = 163
	KEY_PREVIOUSSONG = 165
	KEY_F11         = 0x01dc
	KEY_NUMERIC_0   = 0x0200

	KEY_OK    = 352
	KEY_LEFT  = 105
	KEY_RIGHT = 106
	KEY_MENU  = 139
	KEY_UP    = 103
	KEY_DOWN  = 108

	KEY_FORWARD = 0x009f
	KEY_BACK    = 0x009e
	KEY_TIME    = 0x0167
)

// ============================================================
// Favorites
// ============================================================

func (c *Controller) favoriteList() []Action {
	return []Action{
		func() error { return c.playPlaylist("Couleur3") },             // mode 0 key 0
		func() error { return c.playPlaylist("aaa_rts") },
		func() error { return c.artist("System of A Down", "") },
		func() error { return c.artist("Muse", "") },
		func() error { return c.artist("Twenty One Pilots", "") },
		func() error { return c.artist("Rammstein", "") },
		func() error { return c.artist("Marilyn Manson", "") },
		func() error { return c.artist("ACDC", "A.C.D.C") },
		func() error { return c.artist("Lofofora", "") },
		func() error { return c.artist("Korn", "") },
		func() error { return c.artist("Imagine Dragons", "") },       // mode 1 key 0
		func() error { return c.artist("Placebo", "") },
		func() error { return c.artist("Prodigy", "") },
		func() error { return c.artist("Evanescence", "") },
		func() error { return c.artist("IAM", "I.A.M") },
		func() error { return c.artist("Brell", "") },
		func() error { return c.artist("Slipknot", "Slip-Knot") },
		func() error { return c.artist("Noir désir", "") },
		func() error { return c.artist("Architects", "") },
		func() error { return c.artist("Puddle of Mudd", "") },
		func() error { return c.artist("Seether", "") },               // mode 2 key 0
		func() error { return c.artist("Pleymo", "Play Mo") },
		func() error { return c.artist("Soulfy", "Soul-Fly") },
		func() error { return c.artist("Dub Inc", "Dub. Inc") },
	}
}

// ============================================================
// Core helpers
// ============================================================

func sleep(ms int) error {
	fmt.Println("> sleep", ms)
	time.Sleep(time.Duration(ms) * time.Millisecond)
	return nil
}

func (c *Controller) run(cmd string) (string, error) {
	fmt.Println(">", cmd)

	command := exec.Command("sh", "-c", cmd)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	outStr := stdout.String()
	errStr := stderr.String()

	if outStr != "" {
		fmt.Print(outStr)
	}
	if errStr != "" {
		fmt.Fprint(os.Stderr, errStr)
	}

	if err != nil {
		if !strings.Contains(cmd, "espeak") {
			_ = c.say(truncate(strings.ReplaceAll(err.Error(), c.baseDir, ""), 40), false)
		}
		return outStr, err
	}

	return outStr, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func chain(actions ...Action) error {
	for _, a := range actions {
		if err := a(); err != nil {
			return err
		}
	}
	return nil
}

func stripSpecialChars(s string) string {
	t := norm.NFD.String(s)
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, t)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (c *Controller) say(text string, convertSpecialChars bool) error {
	if convertSpecialChars {
		text = stripSpecialChars(text)
	}
	_, err := c.run(`espeak -v mb-us1 ` + shellQuote(text) + ` --stdout | paplay`)
	return err
}

func (c *Controller) sayMpcStatus() error {
	out, err := c.run("mpc status")
	if err != nil {
		return err
	}
	line := ""
	parts := strings.Split(out, "\n")
	if len(parts) > 0 {
		line = strings.TrimSpace(parts[0])
	}
	if line == "" {
		line = "No status"
	}
	return c.say(line, true)
}

func (c *Controller) artist(name, sayname string) error {
	if sayname == "" {
		sayname = name
	}
	return chain(
		func() error { return c.say(sayname, true) },
		func() error {
			_, err := c.run(filepath.Join(c.baseDir, "play.sh") + " " + shellQuote(name))
			return err
		},
	)
}

func (c *Controller) light(argsString string) error {
	return chain(
		func() error { return c.say("light "+argsString, true) },
		func() error {
			_, err := c.run(filepath.Join(c.baseDir, "light.sh") + " " + argsString)
			return err
		},
	)
}

func (c *Controller) bluetooth() error {
	return chain(
		func() error { return c.say("bluetooth", false) },
		func() error {
			_, err := c.run(filepath.Join(c.baseDir, "bluetooth.sh"))
			return err
		},
	)
}

func (c *Controller) combined() error {
	return chain(
		func() error { return c.say("combined", false) },
		func() error {
			_, err := c.run(filepath.Join(c.baseDir, "combined.sh"))
			return err
		},
	)
}

func (c *Controller) playPlaylist(name string) error {
	out, err := c.run("mpc lsplaylists")
	if err != nil {
		return c.say(err.Error(), false)
	}

	var list []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			list = append(list, line)
		}
	}

	found := false
	for _, item := range list {
		if item == name {
			found = true
			break
		}
	}

	if !found {
		msg := fmt.Sprintf(`Invalid playlist "%s" use %v`, name, list)
		fmt.Println(msg)
		return c.say(name+" doesn't exist", false)
	}

	err = chain(
		func() error { return c.say(name, false) },
		func() error { _, err := c.run("mpc clear"); return err },
		func() error { _, err := c.run("mpc load " + shellQuote(name)); return err },
		func() error { return sleep(500) },
		func() error { _, err := c.run("mpc random on"); return err },
		func() error { _, err := c.run("mpc shuffle"); return err },
		func() error { return sleep(500) },
		func() error { _, err := c.run("mpc play"); return err },
	)

	if err != nil {
		return c.say(err.Error(), false)
	}
	return nil
}

// ============================================================
// Artist browsing
// ============================================================

func (c *Controller) artistListLoad(forceLoad bool) error {
	c.mu.Lock()
	needLoad := len(c.artistList) == 0 || forceLoad
	c.mu.Unlock()

	if !needLoad {
		return nil
	}

	out, err := c.run("mpc list artist")
	if err != nil {
		return err
	}

	var artists []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "The "), "the "))
		artists = append(artists, line)
	}

	sort.Slice(artists, func(i, j int) bool {
		return strings.ToLower(artists[i]) < strings.ToLower(artists[j])
	})

	c.mu.Lock()
	c.artistIndex = 0
	c.artistList = artists
	fmt.Println("Loaded", len(c.artistList), "artists")
	c.mu.Unlock()

	return nil
}

func (c *Controller) artistListPlaySelection() error {
	if err := c.artistListLoad(false); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.artistIndex < 0 || c.artistIndex >= len(c.artistList) {
		c.artistIndex = 0
	}

	if len(c.artistList) == 0 {
		return errors.New("invalid selection")
	}

	name := c.artistList[c.artistIndex]
	go func() {
		if err := c.artist(name, ""); err != nil {
			_ = c.say(err.Error(), false)
		}
	}()
	return nil
}

func firstLetterLower(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(strings.ToLower(strings.TrimSpace(s)))
	if len(r) == 0 {
		return ""
	}
	return string(r[0])
}

func (c *Controller) artistListByLetter(increment int) error {
	if err := c.artistListLoad(false); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.artistList) == 0 || c.artistIndex < 0 || c.artistIndex >= len(c.artistList) {
		return nil
	}

	letters := make([]string, len(c.artistList))
	for i, a := range c.artistList {
		letters[i] = firstLetterLower(a)
	}

	currentLetter := letters[c.artistIndex]
	fmt.Println("Current letter: <<", currentLetter, ">>. Index", c.artistIndex)

	n := len(letters)
	i := c.artistIndex

	for step := 0; step < n; step++ {
		i = (i + increment + n) % n
		if letters[i] != currentLetter {
			if increment < 0 {
				for prev := (i - 1 + n) % n; prev != i && letters[prev] == letters[i]; prev = (prev - 1 + n) % n {
					i = prev
				}
			}
			c.artistIndex = i
			fmt.Println("Next letter: <<", letters[c.artistIndex], ">> at index", c.artistIndex, "like", c.artistList[c.artistIndex])
			go func(letter string) {
				_ = c.say(letter, true)
			}(letters[c.artistIndex])
			return nil
		}
	}

	c.artistIndex = 0
	return nil
}

func (c *Controller) artistListByIncrement(number int) error {
	if err := c.artistListLoad(false); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.artistList) == 0 {
		c.artistIndex = 0
		return nil
	}

	c.artistIndex += number
	if c.artistIndex < 0 {
		c.artistIndex = 0
	}
	if c.artistIndex >= len(c.artistList) {
		c.artistIndex = len(c.artistList) - 1
	}

	fmt.Println("index", c.artistIndex, "length", len(c.artistList), "item", c.artistList[c.artistIndex])

	name := c.artistList[c.artistIndex]
	go func() {
		_ = c.say(name, true)
	}()
	return nil
}

func (c *Controller) playByKeyNumber(index int) error {
	favs := c.favoriteList()

	index = int(math.Max(0, math.Min(float64(len(favs)-1), float64(index))))
	if index >= len(favs) || favs[index] == nil {
		return fmt.Errorf("favoriteList must use a function at index %d", index)
	}
	return favs[index]()
}

func (c *Controller) nextMode() error {
	c.mu.Lock()
	c.altMode = (c.altMode + 1) % len(c.modesMap)
	mode := c.altMode
	c.mu.Unlock()
	return c.say(fmt.Sprintf("Mode. %d", mode), false)
}

// ============================================================
// Key mapping
// ============================================================

func (c *Controller) setupMappings() {
	m0 := c.modesMap[0]
	m1 := c.modesMap[1]

	m0[KEY_FORWARD] = func() error { _, err := c.run("mpc seek +5"); return err }
	m0[KEY_BACK] = func() error { _, err := c.run("mpc seek -5"); return err }
	m0[KEY_PAUSE] = func() error { _, err := c.run("mpc pause"); return err }
	m0[KEY_PLAY] = func() error { _, err := c.run("mpc toggle"); return err }
	m0[KEY_NEXTSONG] = func() error { _, err := c.run("mpc next"); return err }
	m0[KEY_PREVIOUSSONG] = func() error { _, err := c.run("mpc prev"); return err }
	m0[KEY_VOLUMEDOWN] = func() error { _, err := c.run("pactl set-sink-volume @DEFAULT_SINK@ -15%"); return err }
	m0[KEY_VOLUMEUP] = func() error { _, err := c.run("pactl set-sink-volume @DEFAULT_SINK@ +15%"); return err }
	m0[KEY_MUTE] = func() error { _, err := c.run("pactl set-sink-mute @DEFAULT_SINK@ toggle"); return err }

	m0[KEY_WAKEUP] = func() error { return c.say("set mode 1 first", false) }
	m0[KEY_VIDEO_NEXT] = c.nextMode
	m0[KEY_F11] = func() error { _, err := c.run("mpc random on"); return err }
	m0[KEY_STOP] = func() error { _, err := c.run("mpc stop"); return err }
	m0[KEY_INFO] = func() error {
		return chain(
			func() error { return c.bluetooth() },
			func() error { return c.combined() },
		)
	}
	m0[KEY_EJECT] = func() error {
		_, err := c.run(filepath.Join(c.baseDir, "play.sh") + " random 1")
		return err
	}

	m0[KEY_MENU] = func() error {
		if err := c.artistListLoad(true); err != nil {
			return err
		}
		c.mu.Lock()
		nb := len(c.artistList)
		c.mu.Unlock()
		return c.say(fmt.Sprintf("Reloaded %d artists", nb), false)
	}
	m0[KEY_RIGHT] = func() error { return c.artistListByIncrement(+1) }
	m0[KEY_LEFT] = func() error { return c.artistListByIncrement(-1) }
	m0[KEY_UP] = func() error { return c.artistListByLetter(-1) }
	m0[KEY_DOWN] = func() error { return c.artistListByLetter(+1) }
	m0[KEY_OK] = func() error { return c.artistListPlaySelection() }
	m0[KEY_TIME] = func() error { return c.sayMpcStatus() }

	// Power off only on mode 1 for safety
	m1[KEY_WAKEUP] = func() error {
		if err := c.say("Powering off", false); err != nil {
			return err
		}
		_, err := c.run("sudo systemctl poweroff")
		return err
	}
	m1[KEY_PAUSE] = func() error { return c.light("scene_recall 4") }
	m1[KEY_PLAY] = func() error { return c.light("scene_recall 8") }
	m1[KEY_STOP] = func() error { return c.light("state on") }
	m1[KEY_EJECT] = func() error { return c.light("state off") }

	// Numeric keys in each mode
	for modeIndex, m := range c.modesMap {
		for key := 0; key < 10; key++ {
			idx := modeIndex*10 + key
			keyCode := uint16(KEY_NUMERIC_0 + key)
			m[keyCode] = func(index int) Action {
				return func() error {
					return c.playByKeyNumber(index)
				}
			}(idx)
		}
	}
}

// ============================================================
// Event loop
// ============================================================

func (c *Controller) handleKey(code uint16, value int32) {
	// JS did "if (!value) return", which means only react to non-zero values.
	// Linux key events are usually:
	// 0 = release, 1 = press, 2 = repeat
	if value == 0 {
		return
	}

	c.mu.Lock()
	mode := c.altMode
	c.mu.Unlock()

	commands := c.modesMap[mode]

	var action Action
	if a, ok := commands[code]; ok {
		action = a
	} else if a, ok := c.modesMap[0][code]; ok {
		action = a
	}

	if action != nil {
		go func() {
			if err := action(); err != nil {
				_ = c.say(err.Error(), false)
			}
		}()
		return
	}

	fmt.Println("State is now:", value, "for key", code, "mode", mode)
}

func (c *Controller) listen(device string) error {
	f, err := os.Open(device)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", device, err)
	}
	defer f.Close()

	fmt.Println("Listening on", device)

	for {
		var ev inputEvent
		err := binary.Read(f, binary.LittleEndian, &ev)
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return fmt.Errorf("failed to read input event: %w", err)
		}

		if ev.Type == EV_KEY {
			c.handleKey(ev.Code, ev.Value)
		}
	}
}
