package cli

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
)

type styler struct {
	ok    func(string) string
	warn  func(string) string
	err   func(string) string
	plain bool
}

func newStyler(w io.Writer) styler {
	out, ok := w.(*os.File)
	if !ok || !isatty.IsTerminal(out.Fd()) {
		return styler{plain: true}
	}
	p := termenv.ColorProfile()
	return styler{
		ok:   termenv.String().Foreground(p.Color("#4caf50")).Styled,
		warn: termenv.String().Foreground(p.Color("#ff9800")).Styled,
		err:  termenv.String().Foreground(p.Color("#e53935")).Styled,
	}
}

func (s styler) OK(msg string) string {
	if s.plain || s.ok == nil {
		return msg
	}
	return s.ok(msg)
}

func (s styler) Warn(msg string) string {
	if s.plain || s.warn == nil {
		return msg
	}
	return s.warn(msg)
}

func (s styler) Err(msg string) string {
	if s.plain || s.err == nil {
		return msg
	}
	return s.err(msg)
}
