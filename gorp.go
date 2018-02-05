package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/iriri/minimal/color"
	"github.com/iriri/minimal/flag"
	"github.com/iriri/minimal/gitignore"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

type flagSet struct {
	// A     int
	// B     int
	// C     int
	I bool
	g bool
	i bool
	n bool
	r bool
	v bool
	// bcpl  bool
	color  bool
	fibers int
	git    bool
}

func parseFlags() (int, *flagSet) {
	var opt flagSet
	// flag.Int(&opt.A, 0, "print lines after each match", "", 'A')
	// flag.Int(&opt.B, 0, "print lines before each match", "", 'B')
	// flag.Int(&opt.C, 0, "print lines around each match", "", 'C')
	flag.Bool(&opt.I, false, "ignore binary files", "", 'I')
	flag.Bool(&opt.g, false, "ignore files in .gorpignore", "", 'g')
	flag.Bool(&opt.i, false, "case insensitive matching", "", 'i')
	flag.Bool(&opt.n, false, "print filenames and line numbers", "", 'n')
	flag.Bool(&opt.r, false, "gorp directories recursively", "", 'r')
	flag.Bool(&opt.v, false, "invert match", "", 'v')
	// flag.Bool(&opt.bcpl, false, "curly brace mode", "bcpl", 0)
	flag.Bool(&opt.color, false, "highlight matches", "color", 0)
	flag.Int(&opt.fibers, 4, "files to search concurrently", "fibers", 0)
	flag.Bool(&opt.git, false, "ignore files in .gitignore", "git", 0)
	return flag.Parse(1), &opt
}

func isPiped() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		panic(err)
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func setOptions(first int, opt *flagSet) (*regexp.Regexp, []string) {
	var s string
	if opt.i {
		s = strings.ToLower(os.Args[first])
	} else {
		s = os.Args[first]
	}
	regex, err := regexp.Compile(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}
	if isPiped() {
		return regex, os.Args[0:1]
	}

	var ign gitignore.Ignore
	if opt.g {
		if _, err := os.Stat(".gorpignore"); err == nil {
			ign, _ = gitignore.From(".gorpignore")
		}
	}
	if opt.git {
		ign = gitignore.New()
		if _, err := os.Stat(".gitignore"); err == nil {
			ign.Append(".gitignore")
		}
		if _, err := os.Stat("/.gitignore_global"); err == nil {
			ign.Append("/.gitignore_global")
		}
	}

	if opt.r {
		fnames := make([]string, 0, len(os.Args[first+1:])*4)
		fn := func(path string, info os.FileInfo, err error) error {
			fnames = append(fnames, path)
			return err
		}
		for _, s := range os.Args[first+1:] {
			if ign != nil {
				err = gitignore.Walk(s, ign, false, fn)
			} else {
				err = filepath.Walk(s, fn)
			}
			if err != nil {
				panic(err)
			}
		}
		return regex, fnames
	} else if ign != nil {
		fnames := make([]string, 0, len(os.Args[first+1:]))
		for _, s := range os.Args[first+1:] {
			if !ign.Match(s) {
				fnames = append(fnames, s)
			}
		}
		return regex, fnames
	}

	return regex, os.Args[first+1:]
}

func isBinary(f *os.File) bool {
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return true
	}
	f.Seek(0, 0)
	return !utf8.Valid(buf[:n])
}

func scanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func colorize(s string) string {
	return color.Cyan + s + color.Reset
}

func match(r *regexp.Regexp, fname string, opt *flagSet, c chan string,
	scn *bufio.Scanner) {
	var l string
	s := []string{fname, ":", "", ": ", ""}
	if fname == "" {
		s[1] = ""
	}
	for n := 1; scn.Scan(); n++ {
		if opt.i {
			l = strings.ToLower(scn.Text())
		} else {
			l = scn.Text()
		}
		if r.MatchString(l) != opt.v {
			if opt.color {
				if opt.n {
					s[2] = strconv.Itoa(n)
					s[4] = r.ReplaceAllStringFunc(l, colorize)
					c <- strings.Join(s, "")
				} else {
					c <- r.ReplaceAllStringFunc(l, colorize)
				}
			} else if opt.n {
				s[2] = strconv.Itoa(n)
				s[4] = scn.Text()
				c <- strings.Join(s, "")
			} else {
				c <- scn.Text()
			}
		}
	}
	close(c)
}

func search(r *regexp.Regexp, fname string, opt *flagSet, c chan string) {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening %s: %s\n", fname,
			err.(*os.PathError).Err)
		close(c)
		return
	}
	defer f.Close()
	if opt.I && isBinary(f) {
		close(c)
		return
	}

	scn := bufio.NewScanner(bufio.NewReader(f))
	scn.Split(scanLines)
	match(r, fname, opt, c, scn)
}

func write(cc chan chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for c := range cc {
		for s := range c {
			w.WriteString(s)
		}
	}
}

func main() {
	first, opt := parseFlags()
	regex, fnames := setOptions(first, opt)

	cc := make(chan chan string, opt.fibers)
	var wg sync.WaitGroup
	wg.Add(1)
	go write(cc, &wg)
	if isPiped() {
		c := make(chan string, 128)
		scn := bufio.NewScanner(bufio.NewReader(os.Stdin))
		scn.Split(scanLines)
		go match(regex, "", opt, c, scn)
		cc <- c
	} else {
		for _, s := range fnames {
			c := make(chan string, 128)
			cc <- c
			go search(regex, s, opt, c)
		}
	}
	close(cc)
	wg.Wait()
}
