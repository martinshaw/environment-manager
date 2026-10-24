package tui

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// artPalette indices are chosen by the scene, not cycled at random.
// 0-4 gold, 5-9 cyan, 10-14 magenta, 15-19 dragon green, 20-24 fire, 25 shadow.
var artPalette = []lipgloss.Color{
	"#fffbeb", "#fde68a", "#fbbf24", "#f59e0b", "#d97706",
	"#ecfeff", "#a5f3fc", "#22d3ee", "#06b6d4", "#0e7490",
	"#fdf4ff", "#f0abfc", "#e879f9", "#c026d3", "#86198f",
	"#ecfdf5", "#6ee7b7", "#34d399", "#059669", "#065f46",
	"#fff7ed", "#fdba74", "#fb923c", "#ef4444", "#9f1239",
	"#4c1d95",
}

const artBlank = -1

type artCell struct {
	ch    rune
	color int
}

// Ornate block letters. Every row of a letter is the same width.
var letterE = []string{
	"██████████",
	"██      ██",
	"██  ██████",
	"██  ██████",
	"██        ",
	"██      ██",
	"██████████",
}
var letterN = []string{
	"██      ██",
	"███     ██",
	"██ ██   ██",
	"██  ██  ██",
	"██   ██ ██",
	"██    ████",
	"██      ██",
}
var letterV = []string{
	"██      ██",
	"██      ██",
	" ██    ██ ",
	" ██    ██ ",
	"  ██  ██  ",
	"   ████   ",
	"    ██    ",
}
var letterM = []string{
	"██      ██",
	"███    ███",
	"████  ████",
	"██ ████ ██",
	"██  ██  ██",
	"██      ██",
	"██      ██",
}

// Dragons face right. '!' is a backslash. The jaw row ends at the open mouth.
var dragonSmall = slashArt(
	"                 /^!",
	"                //  !",
	"      __       // @@ !",
	"   .-'  `-.   // !_/ !",
	"  /   ____  !/       )",
	" |   /    !  |      /",
	" |  |  ^^  | |     /",
	"  !  !    ! |     /",
	"   !  `---' |    /",
	"    `._    _|   /",
	"       |  |    /",
	"  _____|  |___/",
	" /     |  |    !",
	"/_____/    !____!",
)

const dragonSmallMouth = 4

var dragonBig = slashArt(
	"                                    /^!",
	"                                   //  !",
	"                                  // @@ !",
	"                       __        // !_/ !",
	"                    .-'  `-.    //       !",
	"                   /        !_ /         )",
	"                  |   ____   |          /",
	"                  |  /    !  |         /",
	"                  | |  ^^  | |        /",
	"                   ! !    ! |        /",
	"                    ! `---' |       /",
	"                     `._   _|      /",
	"                        | |       /",
	"                   _____| |______/",
	"                  /     | |       !",
	"                 /     /   !       !",
	"                /_____/     !_______!",
	"                      !     /",
	"                       !___/",
)

const dragonBigMouth = 5

// backWing sits on the large dragon's shoulder.
var backWing = slashArt(
	"       /^!",
	"      /   !",
	"     /     !",
	"    /       !",
	"   /         !",
	"  /___________!",
)

var wing = []string{
	`            /|`,
	`           //|`,
	`          ///|`,
	`         ////|`,
	`        /////|`,
	`       //////|`,
	`      ///////|`,
	`     ////////|`,
	`    /////////|`,
	`   //////////|`,
}

func (m Model) passScreen() string {
	modal := passModalStyle().Width(min(m.width-4, 72)).Render(m.passBody())
	return overlayCenter(passphraseArt(m.width, m.height), modal, m.width, m.height)
}

func passModalStyle() lipgloss.Style {
	bg := lipgloss.Color("#14121c")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#67e8f9")).
		Background(bg).
		BorderBackground(bg).
		Padding(1, 2)
}

func passphraseArt(width, height int) []string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	grid := make([][]artCell, height)
	for y := range grid {
		grid[y] = make([]artCell, width)
		for x := range grid[y] {
			grid[y][x] = artCell{ch: ' ', color: artBlank}
		}
	}
	paintAurora(grid, 0, min(4, height))
	paintAurora(grid, max(0, height-3), height)
	sprinkle(grid)
	paintLogo(grid)
	paintWings(grid)
	paintDragon(grid)
	lines := make([]string, height)
	for y := range grid {
		lines[y] = renderArtRow(grid[y])
	}
	return lines
}

func paintLogo(grid [][]artCell) {
	h, w := len(grid), len(grid[0])
	words := joinArt(3, letterE, letterN, letterV, letterM)
	logo := frameArt(words)
	lw := lipWidth(logo)
	if h < 20 || w < lw+4 {
		return
	}
	y := 1
	if h >= 34 {
		y = 3
	}
	x := (w - lw) / 2
	clearRect(grid, x-2, y-1, lw+6, len(logo)+4)
	stampSolid(grid, x, y, logo, 3)
	stampBand(grid, x+2, y+1, words, []int{5, 6, 7, 8, 2, 11, 12, 13})
	punchSpaces(grid, x, y, logo)
	dropShadow(grid, x, y, lw, len(logo))
	if y >= 2 {
		stampSolid(grid, x, y-1, []string{crown(lw)}, 3)
	}
	paintBursts(grid, x, y, lw, len(logo))
}

func frameArt(rows []string) []string {
	w := lipWidth(rows)
	bar := strings.Repeat("═", w+2)
	out := make([]string, 0, len(rows)+2)
	out = append(out, "╔"+bar+"╗")
	for _, row := range rows {
		pad := w - lipgloss.Width(row)
		if pad < 0 {
			pad = 0
		}
		out = append(out, "║ "+row+strings.Repeat(" ", pad)+" ║")
	}
	out = append(out, "╚"+bar+"╝")
	return out
}

func dropShadow(grid [][]artCell, x, y, w, h int) {
	for dy := 1; dy <= h; dy++ {
		shade(grid, x+w, y+dy)
	}
	for dx := 1; dx <= w; dx++ {
		shade(grid, x+dx, y+h)
	}
}

func shade(grid [][]artCell, x, y int) {
	if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[0]) {
		return
	}
	if grid[y][x].ch != ' ' {
		return
	}
	grid[y][x] = artCell{ch: '▓', color: 25}
}

var burst = slashArt(
	"    +",
	"    |",
	" !  |  /",
	"--- + ---",
	" /  |  !",
	"    |",
	"    +",
)

func paintBursts(grid [][]artCell, logoX, logoY, logoW, logoH int) {
	bw := lipWidth(burst)
	y := logoY + max(0, logoH/2-len(burst)/2)
	if left := logoX - bw - 3; left >= 0 {
		stampSolid(grid, left, y, burst, 11)
	}
	if right := logoX + logoW + 3; right+bw < len(grid[0]) {
		stampSolid(grid, right, y, mirrorArt(burst), 7)
	}
}

func crown(width int) string {
	spike := "/^\\ "
	n := width / len(spike)
	if n < 1 {
		return strings.Repeat("*", max(0, width))
	}
	body := strings.Repeat(spike, n)
	pad := width - len(body)
	left := pad / 2
	return strings.Repeat(" ", left) + body + strings.Repeat(" ", pad-left)
}

func paintWings(grid [][]artCell) {
	h, w := len(grid), len(grid[0])
	ww := lipWidth(wing)
	// The card is about 78 columns. Wings only show in the leftover margin.
	margin := (w - 78) / 2
	if h < 24 || margin < ww-4 {
		return
	}
	y := h/2 - len(wing)/2
	stampBand(grid, 1, y, wing, []int{10, 11, 12, 13, 14})
	stampBand(grid, w-ww-1, y, mirrorArt(wing), []int{14, 13, 12, 11, 10})
}

func paintDragon(grid [][]artCell) {
	h, w := len(grid), len(grid[0])
	if h < 22 || w < 40 {
		breatheFire(grid, 0, h-2, w, 2)
		return
	}
	sprite := dragonSmall
	mouth := dragonSmallMouth
	flames := 4
	if h >= 48 {
		sprite = dragonBig
		mouth = dragonBigMouth
		flames = 7
	}
	y := (h + 14) / 2
	if y+len(sprite) > h-1 {
		y = h - len(sprite) - 1
	}
	if y < 1 {
		y = 1
	}
	clearRect(grid, 1, y, lipWidth(sprite)+2, len(sprite))
	if len(sprite) == len(dragonBig) {
		stampFn(grid, 4, y+5, backWing, dragonColor(w))
	}
	stampFn(grid, 1, y, sprite, dragonColor(w))
	jaw := strings.TrimRight(sprite[mouth], " ")
	breatheFire(grid, 1+lipgloss.Width(jaw), y+mouth, w, flames)
	riseEmbers(grid, y)
}

// breatheFire paints a jet that starts hot at x and cools toward the right edge.
func breatheFire(grid [][]artCell, x, y, width, rows int) {
	glyphs := []struct {
		ch    rune
		color int
	}{
		{'#', 24}, {'@', 23}, {'O', 23}, {'o', 22}, {'*', 22}, {'^', 21}, {'~', 21}, {'.', 20},
	}
	for dy := 0; dy < rows; dy++ {
		yy := y + dy
		if yy < 0 || yy >= len(grid) {
			continue
		}
		for dx := dy * 2; x+dx < width && x+dx < len(grid[yy]); dx++ {
			if !canBurn(grid[yy][x+dx]) || (dx > 14 && (dx+dy*2)%11 == 0) {
				continue
			}
			heat := dx / 4
			if heat >= len(glyphs) {
				heat = len(glyphs) - 1
			}
			if dy == rows-1 && dx%3 != 0 {
				continue
			}
			g := glyphs[heat]
			grid[yy][x+dx] = artCell{ch: g.ch, color: g.color}
		}
	}
}

func canBurn(c artCell) bool {
	switch c.ch {
	case ' ', '~', '-', '.', '*', '+', '▓':
		return true
	default:
		return false
	}
}

func riseEmbers(grid [][]artCell, dragonTop int) {
	w := len(grid[0])
	for i := 0; i < w/3; i++ {
		x := (i*17 + 3) % w
		y := dragonTop - 1 - (i*5)%max(1, dragonTop/2)
		if y < 0 || y >= len(grid) || grid[y][x].ch != ' ' {
			continue
		}
		ch := '.'
		color := 20
		if i%4 == 0 {
			ch = '*'
			color = 22
		} else if i%3 == 0 {
			ch = '^'
			color = 21
		}
		grid[y][x] = artCell{ch: rune(ch), color: color}
	}
}

func dragonColor(width int) func(xx, yy int, r rune) int {
	scales := []int{19, 18, 17, 16}
	return func(xx, _ int, r rune) int {
		switch r {
		case '@':
			return 2
		case '^':
			return 0
		default:
			if width < 1 {
				return scales[0]
			}
			return scales[(xx*len(scales))/width]
		}
	}
}

// slashArt turns '!' into a backslash so sprites can live in raw strings.
func slashArt(rows ...string) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = strings.ReplaceAll(row, "!", `\`)
	}
	return out
}

func punchSpaces(grid [][]artCell, x, y int, rows []string) {
	for dy, row := range rows {
		yy := y + dy
		if yy < 0 || yy >= len(grid) {
			continue
		}
		xx := x
		for _, r := range row {
			if xx >= 0 && xx < len(grid[yy]) && r == ' ' {
				grid[yy][xx] = artCell{ch: ' ', color: artBlank}
			}
			xx++
		}
	}
}

func joinArt(gap int, parts ...[]string) []string {
	h := len(parts[0])
	out := make([]string, h)
	pad := strings.Repeat(" ", gap)
	for i, part := range parts {
		for row := 0; row < h; row++ {
			out[row] += part[row]
			if i < len(parts)-1 {
				out[row] += pad
			}
		}
	}
	return out
}

func mirrorArt(rows []string) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		r := []rune(row)
		for a, b := 0, len(r)-1; a < b; a, b = a+1, b-1 {
			r[a], r[b] = flipRune(r[b]), flipRune(r[a])
		}
		out[i] = string(r)
	}
	return out
}

func flipRune(r rune) rune {
	switch r {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '(':
		return ')'
	case ')':
		return '('
	case '<':
		return '>'
	case '>':
		return '<'
	default:
		return r
	}
}

func paintAurora(grid [][]artCell, y0, y1 int) {
	if len(grid) == 0 {
		return
	}
	w := len(grid[0])
	for y := y0; y < y1 && y < len(grid); y++ {
		for x := 0; x < w; x++ {
			if grid[y][x].ch != ' ' {
				continue
			}
			s := math.Sin(float64(x)/4.5 + float64(y)*0.8)
			var ch rune
			switch {
			case s > 0.55:
				ch = '~'
			case s > 0.2:
				ch = '-'
			case s < -0.65:
				ch = '.'
			default:
				continue
			}
			grid[y][x] = artCell{ch: ch, color: (x/2 + y) % len(artPalette)}
		}
	}
}

func sprinkle(grid [][]artCell) {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return
	}
	h, w := len(grid), len(grid[0])
	n := w * h / 40
	if n > 400 {
		n = 400
	}
	for i := 0; i < n; i++ {
		x := (i*73 + 11) % w
		y := (i*29 + 5) % h
		if grid[y][x].ch != ' ' {
			continue
		}
		ch := '.'
		switch {
		case i%11 == 0:
			ch = '+'
		case i%7 == 0:
			ch = '*'
		}
		grid[y][x] = artCell{ch: rune(ch), color: (i + x) % len(artPalette)}
	}
}

func clearRect(grid [][]artCell, x, y, w, h int) {
	for dy := 0; dy < h; dy++ {
		yy := y + dy
		if yy < 0 || yy >= len(grid) {
			continue
		}
		for dx := 0; dx < w; dx++ {
			xx := x + dx
			if xx < 0 || xx >= len(grid[yy]) {
				continue
			}
			grid[yy][xx] = artCell{ch: ' ', color: artBlank}
		}
	}
}

func stampSolid(grid [][]artCell, x, y int, rows []string, color int) {
	stampFn(grid, x, y, rows, func(int, int, rune) int { return color })
}

func stampBand(grid [][]artCell, x, y int, rows []string, colors []int) {
	stampFn(grid, x, y, rows, func(xx, _ int, _ rune) int {
		if len(colors) == 0 {
			return 0
		}
		w := len(grid[0])
		if w < 1 {
			w = 1
		}
		return colors[(xx*len(colors))/w]
	})
}

func stampFn(grid [][]artCell, x, y int, rows []string, color func(xx, yy int, r rune) int) {
	for dy, row := range rows {
		yy := y + dy
		if yy < 0 || yy >= len(grid) {
			continue
		}
		xx := x
		for _, r := range row {
			if xx >= 0 && xx < len(grid[yy]) && r != ' ' {
				grid[yy][xx] = artCell{ch: r, color: color(xx, yy, r)}
			}
			xx++
		}
	}
}

func renderArtRow(cells []artCell) string {
	var b strings.Builder
	for i := 0; i < len(cells); {
		j := i + 1
		for j < len(cells) && cells[j].color == cells[i].color {
			j++
		}
		var chunk strings.Builder
		for _, c := range cells[i:j] {
			chunk.WriteRune(c.ch)
		}
		text := chunk.String()
		if cells[i].color == artBlank {
			b.WriteString(text)
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(artPalette[cells[i].color%len(artPalette)]).Render(text))
		}
		i = j
	}
	return b.String()
}

func lipWidth(rows []string) int {
	w := 0
	for _, row := range rows {
		if n := lipgloss.Width(row); n > w {
			w = n
		}
	}
	return w
}

// overlayCenter paints fg in the middle of a full-screen background.
func overlayCenter(bg []string, fg string, width, height int) string {
	if width < 1 || height < 1 {
		return fg
	}
	for len(bg) < height {
		bg = append(bg, strings.Repeat(" ", width))
	}
	fgLines := strings.Split(fg, "\n")
	for len(fgLines) > 0 && fgLines[len(fgLines)-1] == "" {
		fgLines = fgLines[:len(fgLines)-1]
	}
	fgW := 0
	for _, line := range fgLines {
		if n := lipgloss.Width(line); n > fgW {
			fgW = n
		}
	}
	if fgW > width {
		fgW = width
	}
	top := (height - len(fgLines)) / 2
	left := (width - fgW) / 2
	if top < 0 {
		top = 0
	}
	if left < 0 {
		left = 0
	}
	card := lipgloss.NewStyle().Background(lipgloss.Color("#14121c"))
	out := make([]string, height)
	for y := 0; y < height; y++ {
		line := bg[y]
		fi := y - top
		if fi >= 0 && fi < len(fgLines) {
			piece := fgLines[fi]
			if lipgloss.Width(piece) > fgW {
				piece = ansi.Truncate(piece, fgW, "")
			}
			if gap := fgW - lipgloss.Width(piece); gap > 0 {
				piece += card.Render(strings.Repeat(" ", gap))
			}
			line = spliceArt(line, left, piece, width)
		}
		if lipgloss.Width(line) > width {
			line = ansi.Truncate(line, width, "")
		}
		out[y] = line
	}
	return strings.Join(out, "\n")
}

// spliceArt replaces the visual columns [at, at+width(piece)) of an ANSI line.
func spliceArt(line string, at int, piece string, width int) string {
	left := ansi.Truncate(line, at, "")
	rightStart := at + lipgloss.Width(piece)
	if rightStart > width {
		rightStart = width
	}
	return left + piece + cutLeft(line, rightStart)
}

// cutLeft drops the first n visual columns and keeps the color of the run
// that crosses the cut.
func cutLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	var b strings.Builder
	var open string
	width := 0
	skipping := true
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			seq, size := readANSI(s[i:])
			if skipping {
				open = seq
			} else {
				b.WriteString(seq)
			}
			i += size
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if skipping {
			width += lipgloss.Width(string(r))
			if width >= n {
				skipping = false
				b.WriteString(open)
			}
			i += size
			continue
		}
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

func readANSI(s string) (string, int) {
	if len(s) == 0 || s[0] != 0x1b {
		return "", 0
	}
	if len(s) > 1 && s[1] == '[' {
		i := 2
		for i < len(s) {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				return s[:i+1], i + 1
			}
			i++
		}
		return s, len(s)
	}
	if len(s) > 1 {
		return s[:2], 2
	}
	return s, 1
}
