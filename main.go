package main

import "github.com/cyb33rr/rtlog/cmd"

func main() {
	cmd.SetEmbeddedFiles(embeddedFiles)
	cmd.Execute()
}
