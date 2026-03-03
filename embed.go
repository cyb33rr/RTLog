package main

import "embed"

//go:embed hook.zsh tools.conf extract.conf
var embeddedFiles embed.FS
