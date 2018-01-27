package main

import (
	"bufio"
	"fmt"
	"io"
	// "github.com/iriri/minimal/color" // more NIH syndrome coming soon
	"github.com/iriri/minimal/flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

type flagSet struct {
	A    bool // not implemented
	B    bool // not implemented
	C    bool // not implemented
	i    bool
	I    bool
	n    bool
	r    bool
	bcpl bool // not implemented
}

func parseFlags() (int, *flagSet) {
	var flags flagSet
	flag.Bool(&flags.A, false, "print lines after each match", "", 'i')
	flag.Bool(&flags.B, false, "print lines before each match", "", 'i')
	flag.Bool(&flags.C, false, "print lines around each match", "", 'i')
	flag.Bool(&flags.i, false, "case insensitive matching", "", 'i')
	flag.Bool(&flags.I, false, "ignore binary files", "", 'I')
	flag.Bool(&flags.n, false, "print filenames and line numbers", "", 'n')
	flag.Bool(&flags.r, false, "gorp directories recursively", "", 'r')
	flag.Bool(&flags.bcpl, false, "curly brace mode", "bcpl", 0)
	return flag.Parse(1), &flags
}

func setOptions(first int, flags *flagSet) (*regexp.Regexp, []string) {
	var s string
	if flags.i {
		s = strings.ToLower(os.Args[first])
	} else {
		s = os.Args[first]
	}
	regex, err := regexp.Compile(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}

	if flags.r {
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

func search(r *regexp.Regexp, fname string, flags *flagSet, c chan string) {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
		panic(err)
	}
	defer f.Close()
	defer close(c)
	if flags.I && isBinary(f) {
		return
	}

	scn := bufio.NewScanner(bufio.NewReader(f))
	var l string
	s := []string{fname, " ", "", ": ", ""}
	for n := 1; scn.Scan(); n++ {
		if flags.i {
			l = strings.ToLower(scn.Text())
		} else {
			l = scn.Text()
		}
		if r.FindString(l) != "" {
			if flags.n {
				s[2] = strconv.Itoa(n)
				s[4] = scn.Text()
				c <- strings.Join(s, "")
			} else {
				c <- scn.Text()
			}
		}
	}
}

func main() {
	first, flags := parseFlags()
	regex, fnames := setOptions(first, flags)

	cs := make([]chan string, len(fnames))
	for i, s := range fnames {
		cs[i] = make(chan string, 128)
		go search(regex, s, flags, cs[i])
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
