package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func search(r *regexp.Regexp, s *bufio.Scanner, c chan string) {
	for s.Scan() {
		if r.FindString(s.Text()) != "" {
			c <- s.Text()
		}
	}
	close(c)
}

func main() {
	r, err := regexp.Compile(strings.Replace(os.Args[1], "\\|", "|", -1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regexp, probably\n")
		panic(err)
	}

	var f *os.File
	var scn *bufio.Scanner
	cs := make([]chan string, len(os.Args)-2)
	for i, s := range os.Args[2:] {
		f, err = os.Open(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
			panic(err)
		}
		defer f.Close()

		scn = bufio.NewScanner(bufio.NewReader(f))
		cs[i] = make(chan string)
		go search(r, scn, cs[i])
	}

	for _, c := range cs {
		for s := range c {
			fmt.Println(s)
		}
	}
}
