package main

import (
	"bufio"
	"fmt"
	"github.com/iriri/minimal/flag"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type flagSet struct {
	i bool
	n bool
	r bool
}

func parseFlags() (int, *flagSet) {
	var flags flagSet
	flag.Bool(&flags.i, false, "case insensitive matching", "", 'i')
	flag.Bool(&flags.n, false, "print filenames and line numbers", "", 'n')
	flag.Bool(&flags.r, false, "gorp directories recursively", "", 'r')
	return flag.Parse(1), &flags
}

func search(r *regexp.Regexp, fname string, flags *flagSet, c chan string) {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
		panic(err)
	}
	scn := bufio.NewScanner(bufio.NewReader(f))
	defer f.Close()

	s := []string{fname, " ", "", ": ", ""}
	for n := 1; scn.Scan(); n++ {
		if r.FindString(scn.Text()) != "" {
			if flags.n {
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

func main() {
	first, flags := parseFlags()

	regex, err := regexp.Compile(os.Args[first])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}

        cs := make([]chan string, len(os.Args[first + 1:]))
	for i, s := range os.Args[first + 1:] {
		cs[i] = make(chan string, 128)
		go search(regex, s, flags, cs[i])
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
