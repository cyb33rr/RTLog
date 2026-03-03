package main

import "embed"

//go:embed hook.zsh tools.conf
var embeddedFiles embed.FS
