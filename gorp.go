package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/iriri/minimal/color"
	"github.com/iriri/minimal/flag"
	"github.com/iriri/minimal/gitignore"
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
	x bool
	// bcpl   bool
	color     bool
	fibers    int64
	git       bool
	trim      bool
	charDevIn bool
}

func parseFlags() (int, *flagSet) {
	var opt flagSet
	// flag.Int64(&opt.A, 'A', "", 0, "print lines after each match"
	// flag.Int64(&opt.B, 'B', "", 0, "print lines before each match")
	// flag.Int64(&opt.C, 'C', "", 0, "print lines around each match")
	flag.Bool(&opt.I, 'I', "", false, "ignore binary files")
	flag.Bool(&opt.g, 'g', "", false, "ignore files in .gorpignore")
	flag.Bool(&opt.i, 'i', "", false, "case insensitive matching")
	flag.Bool(&opt.n, 'n', "", false, "print filenames and line numbers")
	flag.Bool(&opt.r, 'r', "", false, "gorp directories recursively")
	flag.Bool(&opt.v, 'v', "", false, "invert match")
	flag.Bool(&opt.x, 'x', "", false, "match whole lines only")
	// flag.Bool(&opt.bcpl, 0, "bcpl", false, "curly brace mode")
	flag.Bool(&opt.color, 0, "color", false, "highlight matches")
	flag.Int64(&opt.fibers, 0, "fibers", 4, "files to search concurrently")
	flag.Bool(&opt.git, 0, "git", false, "ignore files in .gitignore")
	flag.Bool(&opt.trim, 0, "trim", false, "trim whitespace")
	return flag.Parse(1), &opt
}

func isCharDevice(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		panic(err)
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func setOptions(first int, opt *flagSet) (*regexp.Regexp, *regexp.Regexp,
	[]string) {
	var regex, iregex *regexp.Regexp
	var err error
	if opt.x {
		os.Args[first] = "^" + os.Args[first] + "\r?\n?$"
	}
	if opt.i {
		regex, err = regexp.Compile(strings.ToLower(os.Args[first]))
		iregex, _ = regexp.Compile("(?i)" + os.Args[first])
	} else {
		regex, err = regexp.Compile(os.Args[first])
		iregex = regex
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(2)
	}
	if !isCharDevice(os.Stdout) {
		opt.color = false
	}
	if !opt.charDevIn {
		return regex, iregex, os.Args[0:1]
	}

	ign, err := gitignore.New()
	if err == nil {
		if opt.git {
			ign.AppendGit()
		}
		if opt.g {
			ign.Append(".gorpignore")
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
				err = ign.Walk(s, fn)
			} else {
				err = filepath.Walk(s, fn)
			}
			if err != nil {
				panic(err)
			}
		}
		return regex, iregex, fnames
	} else if ign != nil {
		fnames := make([]string, 0, len(os.Args[first+1:]))
		for _, s := range os.Args[first+1:] {
			if !ign.Match(s) {
				fnames = append(fnames, s)
			}
		}
		return regex, iregex, fnames
	}

	return regex, iregex, os.Args[first+1:]
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

func scanLines(data []byte, atEOF bool) (int, []byte, error) {
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

func format(s string, ir *regexp.Regexp, opt *flagSet) string {
	if opt.trim {
		s = strings.Trim(s, " \t")
	}
	if opt.color {
		s = ir.ReplaceAllStringFunc(s, func(s string) string {
			return color.BrightRed + s + color.Reset
		})
	}
	return s
}

func match(r, ir *regexp.Regexp, fname string, opt *flagSet, c chan string,
	scn *bufio.Scanner) {
	var matches bool
	s := []string{fname, ":", "", ": ", ""}
	if fname == "" {
		s[1] = ""
	}
	for n := 1; scn.Scan(); n++ {
		if opt.i {
			matches = r.MatchString(strings.ToLower(scn.Text()))
		} else {
			matches = r.MatchString(scn.Text())
		}
		if matches != opt.v {
			if opt.n {
				s[2] = strconv.Itoa(n)
				s[4] = format(scn.Text(), ir, opt)
				c <- strings.Join(s, "")
			} else {
				c <- format(scn.Text(), ir, opt)
			}
		}
	}
	close(c)
}

func search(r, ir *regexp.Regexp, fname string, opt *flagSet, c chan string) {
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
	match(r, ir, fname, opt, c, scn)
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
	opt.charDevIn = isCharDevice(os.Stdin)
	if len(os.Args) < first+1 ||
		(len(os.Args) < first+2 && opt.charDevIn) {
		fmt.Fprintf(os.Stderr, "not enough arguments\n")
		os.Exit(1)
	}
	regex, iregex, fnames := setOptions(first, opt)

	cc := make(chan chan string, opt.fibers)
	var wg sync.WaitGroup
	wg.Add(1)
	go write(cc, &wg)
	if opt.charDevIn {
		for _, s := range fnames {
			c := make(chan string, 128)
			cc <- c
			go search(regex, iregex, s, opt, c)
		}
	} else {
		c := make(chan string, 128)
		scn := bufio.NewScanner(bufio.NewReader(os.Stdin))
		scn.Split(scanLines)
		go match(regex, iregex, "", opt, c, scn)
		cc <- c
	}
	close(cc)
	wg.Wait()
}
