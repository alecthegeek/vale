package lint

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/errata-ai/regexp2"

	"github.com/errata-ai/vale/v3/internal/core"
	"github.com/errata-ai/vale/v3/internal/glob"
)

var reFrontMatter = regexp.MustCompile(
	`^(?s)(?:---|\+\+\+)\n(.+?)\n(?:---|\+\+\+)`)

var heading = regexp.MustCompile(`^h\d$`)

func (l *Linter) lintHTML(f *core.File) error {
	if l.Manager.Config.Flags.Built != "" {
		return l.lintTxtToHTML(f)
	}
	return l.lintHTMLTokens(f, []byte(f.Content), 0)
}

func (l *Linter) applyPatterns(f *core.File, block, inline string) (string, error) {
	// TODO: Should we assume this?
	s := reFrontMatter.ReplaceAllString(f.Content, block)

	exts := []string{f.NormedExt, f.RealExt}
	for syntax, regexes := range l.Manager.Config.BlockIgnores {
		sec, err := glob.Compile(syntax)
		if err != nil {
			return s, err
		} else if sec.MatchAny(exts) {
			for _, r := range regexes {
				pat, errc := regexp2.CompileStd(r)
				if errc != nil { //nolint:gocritic
					return s, core.NewE201FromTarget(
						errc.Error(),
						r,
						l.Manager.Config.Flags.Path,
					)
				} else if strings.HasSuffix(f.NormedExt, ".rst") {
					// HACK: We need to add padding for the literal block.
					for _, c := range pat.FindAllStringSubmatch(s, -1) {
						sec := fmt.Sprintf(block, core.Indent(c[0], "    "))
						s = strings.Replace(s, c[0], sec, 1)
					}
				} else {
					s, err = pat.Replace(s, block, 0, -1)
					if err != nil {
						return s, core.NewE201FromTarget(
							err.Error(),
							r,
							l.Manager.Config.Flags.Path,
						)
					}
				}
			}
		}
	}

	for syntax, regexes := range l.Manager.Config.TokenIgnores {
		sec, err := glob.Compile(syntax)
		if err != nil {
			return s, err
		} else if sec.MatchAny(exts) {
			for _, r := range regexes {
				pat, errc := regexp2.CompileStd(r)
				if errc != nil {
					return s, core.NewE201FromTarget(
						errc.Error(),
						r,
						l.Manager.Config.Flags.Path,
					)
				}
				s, err = pat.Replace(s, inline, 0, -1)
				if err != nil {
					return s, core.NewE201FromTarget(
						err.Error(),
						r,
						l.Manager.Config.Flags.Path,
					)
				}
			}
		}
	}

	return s, nil
}

func (l *Linter) post(f *core.File, text, url string) (string, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		url,
		bytes.NewBufferString(text))

	if err != nil {
		return "", core.NewE100(f.Path, err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept", "text/plain")

	resp, err := l.client.Do(req)
	if err != nil {
		return "", core.NewE100(f.Path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		return string(body), nil
	}

	return "", core.NewE100(f.Path, errors.New("bad status"))
}

func (l *Linter) lintTxtToHTML(f *core.File) error {
	html, err := os.ReadFile(l.Manager.Config.Flags.Built)
	if err != nil {
		return core.NewE100(f.Path, err)
	}
	return l.lintHTMLTokens(f, html, 0)
}

func ping(domain string) error {
	c1 := make(chan bool, 1)

	go func() {
		for {
			conn, err := net.DialTimeout("tcp", domain, 2*time.Millisecond)
			if err == nil {
				c1 <- true
				conn.Close()
				break
			}
		}
	}()

	select {
	case <-c1:
		return nil
	case <-time.After(500 * time.Millisecond):
		// TODO: How long should this be?
		return errors.New("failed to start server")
	}
}
