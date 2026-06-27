package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Print(runtime.GOMAXPROCS(0))
}
