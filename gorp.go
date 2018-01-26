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

func search(r *regexp.Regexp, s *bufio.Scanner, nf bool, c chan string) {
	n := 0
	ss := make([]string, 3)
	ss[1] = ": "
	for s.Scan() {
		n++
		if r.FindString(s.Text()) != "" {
			if nf {
				ss[0] = strconv.Itoa(n)
				ss[2] = s.Text()
				c <- strings.Join(ss, "")
			} else {
				c <- s.Text()
			}
		}
	}
	close(c)
}

func main() {
	var nf bool
	flag.BoolVar(&nf, "n", false, "displays file names and line numbers")
	flag.Parse()

	r, err := regexp.Compile(strings.Replace(os.Args[2], "\\|", "|", -1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}

	var f *os.File
	var scn *bufio.Scanner
	cs := make([]chan string, len(os.Args)-3)
	for i, s := range os.Args[3:] {
		f, err = os.Open(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
			panic(err)
		}
		defer f.Close()

		scn = bufio.NewScanner(bufio.NewReader(f))
		cs[i] = make(chan string, 128)
		go search(r, scn, nf, cs[i])
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
