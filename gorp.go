package main

import (
	"bufio"
	"fmt"
	"io"
	// "github.com/iriri/minimal/color" // more NIH syndrome coming soon
	"github.com/iriri/minimal/flag"
	"github.com/iriri/minimal/globconv"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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
	// color bool
	git bool
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
	// flag.Bool(&opt.color, false, "highlight matches", "color", 0)
	flag.Bool(&opt.git, false, "ignore files in .gitignore", "git", 0)
	return flag.Parse(1), &opt
}

func appendLines(s string, ss []string) []string {
	f, err := os.Open(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yeah idk why this would happen")
		panic(err)
	}
	scn := bufio.NewScanner(bufio.NewReader(f))
	for scn.Scan() {
		ss = append(ss, globconv.Gtor(scn.Text(), true))
	}
	return ss
}

func filterInput(fnames []string, opt *flagSet) []string {
	if !opt.g && !opt.git {
		return fnames
	}
	ss := make([]string, 0, 32)
	if opt.g {
		if _, err := os.Stat(".gorpignore"); err == nil {
			ss = appendLines(".gorpignore", ss)
		} else if _, err = os.Stat("~/.gorpignore"); err == nil {
			ss = appendLines("~/.gorpignore", ss)
		}
	}
	if opt.git {
		if _, err := os.Stat(".gitignore"); err == nil {
			ss = appendLines(".gitignore", ss)
		} else if _, err := os.Stat("~/.gitignore"); err == nil {
			ss = appendLines("~/.gitignore", ss)
		}
	}

	regex, err := regexp.Compile(strings.Join(ss, "|"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regex in .*ignore file, probably")
		panic(err)
	}
	res := make([]string, 0, len(fnames))
	for _, s := range fnames {
		if regex.FindString(s) == "" {
			res = append(res, s)
		}
	}
	return res
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

	if opt.r {
		fnames := make([]string, 0, len(os.Args[first+1:])*4)
		for _, s := range os.Args[first+1:] {
			err := filepath.Walk(s,
				func(path string, f os.FileInfo,
					err error) error {
					fnames = append(fnames, path)
					return err
				})
			if err != nil {
				panic(err)
			}
		}
		return regex, filterInput(fnames, opt)
	}
	return regex, filterInput(os.Args[first+1:], opt)
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

func match(r *regexp.Regexp, fname string, opt *flagSet, c chan string,
	scn *bufio.Scanner) {
	var l string
	s := []string{fname, " ", "", ": ", ""}
	if fname == "" {
		s[1] = ""
	}
	for n := 1; scn.Scan(); n++ {
		if opt.i {
			l = strings.ToLower(scn.Text())
		} else {
			l = scn.Text()
		}
		if (!opt.v && r.FindString(l) != "") ||
			(opt.v && r.FindString(l) == "") {
			if opt.n {
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
		fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
		panic(err)
	}
	defer f.Close()
	if opt.I && isBinary(f) {
		close(c)
		return
	}

	scn := bufio.NewScanner(bufio.NewReader(f))
	match(r, fname, opt, c, scn)
}

func main() {
	first, opt := parseFlags()
	regex, fnames := setOptions(first, opt)

	cs := make([]chan string, 0, len(fnames))
	if isPiped() {
		cs = append(cs, make(chan string, 16))
		scn := bufio.NewScanner(bufio.NewReader(os.Stdin))
		go match(regex, "", opt, cs[0], scn)
	} else {
		for i, s := range fnames {
			cs = append(cs, make(chan string, 128))
			go search(regex, s, opt, cs[i])
		}
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
