package tui

import "strings"

type commandInfo struct {
	Name  string
	Usage string
	Desc  string
}

func (c commandInfo) display() string {
	if c.Usage != "" {
		return c.Usage
	}
	return c.Name
}

var commands = []commandInfo{
	{Name: "/done", Desc: "review the step"},
	{Name: "/why", Desc: "zoom out"},
	{Name: "/stuck", Desc: "get help"},
	{Name: "/skip", Desc: "next step"},
	{Name: "/recap", Desc: "session recap"},
	{Name: "/run", Usage: "/run [cmd]", Desc: "run code"},
	{Name: "/summary", Desc: "session summary"},
	{Name: "/copy", Desc: "copy session to clipboard"},
	{Name: "/dial", Usage: "/dial <0-3>", Desc: "typing dial"},
	{Name: "/profile", Desc: "adjust profile"},
	{Name: "/help", Desc: "list commands"},
	{Name: "/quit", Desc: "exit"},
}

func matchCommands(text string) []commandInfo {
	if text == "/" {
		return commands
	}
	var out []commandInfo
	for _, c := range commands {
		if strings.HasPrefix(c.Name, text) {
			out = append(out, c)
		}
	}
	return out
}
