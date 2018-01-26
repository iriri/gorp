package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
)

func main() {
	r, err := regexp.Compile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid regex, probably\n")
		panic(err)
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid filepath, probably\n")
		panic(err)
	}

	s := bufio.NewScanner(bufio.NewReader(f))

	for s.Scan() {
		if r.FindString(s.Text()) != "" {
			fmt.Println(s.Text())
		}
	}
}
