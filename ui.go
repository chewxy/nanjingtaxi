package main

import (
	"strings"
	"fmt"
)

func (c *client) uiloop() {
	for s := range c.ui {
		switch {
		case s == " ":
			fmt.Printf("> ")
		case strings.HasPrefix(s, "..."), strings.HasPrefix(s, "\t"):
			fmt.Printf("%s\n", s)

		case strings.HasSuffix(s, ":"):
			fmt.Printf("> %s ", s)
		default:
			fmt.Printf("> %s \n", s)
		}
	}
}