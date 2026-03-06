package main

import "embed"

//go:embed hook.zsh hook.bash bash-preexec.sh tools.conf extract.conf
var embeddedFiles embed.FS
