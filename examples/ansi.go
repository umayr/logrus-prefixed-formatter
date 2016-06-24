package main

import (
	"fmt"
	"github.com/mgutz/ansi"
	"strconv"
)

func main() {
	for i := 0; i <= 256; i++ {
		c := ansi.ColorCode(strconv.Itoa(i))
		fmt.Printf("%sCode: [%d] %s\n", c, i, ansi.Reset)
	}
}
