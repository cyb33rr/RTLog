package main

import "embed"

//go:embed hook.zsh hook.bash hook-noninteractive.zsh hook-noninteractive.bash bash-preexec.sh tools.conf extract.conf
var embeddedFiles embed.FS
