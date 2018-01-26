package main

import (
	"bufio"
	"flag"
	"fmt"
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

func parseFlags() *flagSet {
	var flags flagSet
	flag.BoolVar(&flags.n, "n", false, "displays file names and line numbers")
	flag.Parse()
	return &flags
}

func search(r *regexp.Regexp, fname string, flags *flagSet, c chan string) {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
		panic(err)
	}
	defer f.Close()
	scn := bufio.NewScanner(bufio.NewReader(f))

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
	flags := parseFlags()

	r, err := regexp.Compile(strings.Replace(os.Args[2], "\\|", "|", -1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}

	cs := make([]chan string, len(os.Args)-3)
	for i, s := range os.Args[3:] {
		cs[i] = make(chan string, 128)
		go search(r, s, flags, cs[i])
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
