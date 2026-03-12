package main

import (
	lc "joshborrow.com/lightcurvefiller/pkg"
)

func main() {
	config := lc.ReadConfigFromEnvironment()
	config.Run()
}
