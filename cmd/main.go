package main

import (
	"novel_reader/ui"
	"novel_reader/utils"
)

func main() {
	utils.Main()
	ui.RunApp()
}

/*
	1. Reader
		- Search word in content
	2. Library (folder/directory) could import with finder
		- Grouping
	3. Settings/config file
		- Controls
		- UI
	4. Search Novel (dynamic scrapping)

	Fix:
	1. Too much scroll lag, can't quit
*/
