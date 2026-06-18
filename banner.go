package goxpress

import (
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// Version is the goXpress release version, shown in the startup banner.
const Version = "0.1.0"

// bannerWidth is the inner width (between the vertical borders) of the startup
// banner box.
const bannerWidth = 54

// routeCount returns the total number of registered routes across all methods.
// Each method tree's root priority counts the routes inserted through it.
func (r *Router) routeCount() int {
	var n int
	for _, root := range r.trees {
		n += int(root.priority)
	}
	return n
}

// printBanner writes a Fiber-style startup banner describing the server bound
// to addr. It is a no-op when Banner is disabled.
func (r *Router) printBanner(w io.Writer, addr string) {
	if !r.Banner {
		return
	}

	const (
		topLeft     = "┌"
		topRight    = "┐"
		bottomLeft  = "└"
		bottomRight = "┘"
		horizontal  = "─"
		vertical    = "│"
	)

	// center returns s padded with spaces to bannerWidth, centered.
	center := func(s string) string {
		pad := bannerWidth - len([]rune(s))
		if pad <= 0 {
			return s
		}
		left := pad / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", pad-left)
	}

	// field renders "key ..... value" filling width with dots.
	field := func(key, val string, width int) string {
		dots := max(width-len([]rune(key))-len([]rune(val))-2, 1)
		return key + " " + strings.Repeat(".", dots) + " " + val
	}

	// row wraps content (already padded to bannerWidth) in vertical borders.
	row := func(content string) string {
		return vertical + content + vertical
	}

	// A leading space, two columns and a two-space gutter fill bannerWidth.
	col := (bannerWidth - 3) / 2
	stats := " " +
		field("Handlers", strconv.Itoa(r.routeCount()), col) + "  " +
		field("PID", strconv.Itoa(os.Getpid()), bannerWidth-3-col)

	lines := []string{
		topLeft + strings.Repeat(horizontal, bannerWidth) + topRight,
		row(center("goXpress v" + Version)),
		row(center(displayURL(addr))),
		row(center("")),
		row(stats),
		bottomLeft + strings.Repeat(horizontal, bannerWidth) + bottomRight,
	}

	_, _ = fmt.Fprintln(w, strings.Join(lines, "\n"))
}

// displayURL turns a listen address such as ":3200" or "0.0.0.0:3200" into a
// browsable URL, defaulting an empty or wildcard host to 127.0.0.1.
func displayURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}
