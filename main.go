package main

import (
	"github.com/skrashevich/godiskanal/cmd"
	"github.com/skrashevich/godiskanal/i18n"
)

func main() {
	i18n.Init()
	cmd.Execute()
}
