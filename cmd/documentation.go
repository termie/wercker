//   Copyright © 2016, 2019, Oracle and/or its affiliates.  All rights reserved.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/wercker/wercker/core"
	cli "gopkg.in/urfave/cli.v1"
)

const docPath = "Documentation/command"

var werckerCommandHelpTemplate = `# {{.Name}}

NAME
----
{{.Name}} - {{.Usage}}

USAGE
-----
command {{.Name}}{{if .Flags}} [command options]{{end}} [arguments...]{{if .Description}}

DESCRIPTION
-----------
{{.Description}}{{end}}{{if .Flags}}

OPTIONS
-------

{{range $flag := stringifyFlags $.Flags}}{{Prefixed $flag.Name}}::
  {{$flag.Usage}}{{if $flag.Value}}
  Default;;
    {{$flag.Value}}{{end}}
{{end}}{{end}}`

var werckerAppHelpTemplate = `# {{.Name}}

NAME
----
{{.Name}} - {{.Usage}}

USAGE
-----
  {{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION
-------
{{.Version}}{{if or .Author .Email}}

AUTHOR
------{{if .Author}}
{{.Author}}{{if .Email}} - <{{.Email}}>{{end}}{{else}}
{{.Email}}{{end}}{{end}}

COMMANDS
--------
{{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}::
  {{.Usage}}
{{end}}{{if .Flags}}

GLOBAL OPTIONS
--------------
{{range $flag := stringifyFlags $.Flags}}{{Prefixed $flag.Name}}::
  {{$flag.Usage}}{{if $flag.Value}}
  Default;;
    {{$flag.Value}}{{end}}
{{end}}{{end}}`

// Stringifies the flag and returns the first line.
func shortFlag(flag cli.Flag) string {
	ss := strings.Split(flag.String(), "\n")
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

// setupUsageFormatter configures codegangsta.cli to output usage
// information in the format we want.
func setupUsageFormatter(app *cli.App) {
	cli.CommandHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   command {{.Name}}{{if .Flags}} [command options]{{end}}{{if .ArgsUsage}} {{.ArgsUsage}}{{else}} [arguments...]{{end}}{{if .Description}}

DESCRIPTION:
   {{.Description}}{{end}}{{if .Flags}}

OPTIONS:
{{range .Flags}}{{if not .Hidden}}   {{. | shortFlag}}{{ "\n" }}{{end}}{{end}}{{end}}
`
	cli.AppHelpTemplate = `NAME:
   {{.Name}} - {{.Usage}}

USAGE:
   {{.Name}} {{if .Flags}}[global options] {{end}}command{{if .Flags}} [command options]{{end}} [arguments...]

VERSION:
   {{.Version}}{{if or .Author .Email}}

AUTHOR:{{if .Author}}
  {{.Author}}{{if .Email}} - <{{.Email}}>{{end}}{{else}}
  {{.Email}}{{end}}{{end}}

COMMANDS:
   {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
   {{end}}{{if .Flags}}
GLOBAL OPTIONS:
{{range .Flags}}{{if not .Hidden}}   {{. | shortFlag}}{{ "\n" }}{{end}}{{end}}{{end}}
`

	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		writer := tabwriter.NewWriter(app.Writer, 0, 8, 1, '\t', 0)
		t := template.Must(template.New("help").Funcs(
			template.FuncMap{"shortFlag": shortFlag, "join": strings.Join},
		).Parse(templ))
		err := t.Execute(w, data)
		if err != nil {
			panic(err)
		}
		writer.Flush()
	}
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return
}

func prefixedNames(fullName string) (prefixed string) {
	parts := strings.Split(fullName, ",")
	for i, name := range parts {
		name = strings.Trim(name, " ")
		prefixed += prefixFor(name) + name
		if i < len(parts)-1 {
			prefixed += ", "
		}
	}
	return
}

// stringifyFlags gives us a representation of flags that's usable in templates
func stringifyFlags(flags []cli.Flag) ([]cli.StringFlag, error) {
	usefulFlags := []cli.StringFlag{}
	for _, flag := range flags {
		switch t := flag.(type) {
		default:
			return nil, fmt.Errorf("unexpected type %T", t)
		case cli.StringSliceFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: strings.Join(*t.Value, ","),
			})
		case cli.BoolFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
			})
		case cli.BoolTFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
			})
		case cli.StringFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: t.Value,
			})
		case cli.IntFlag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: fmt.Sprintf("%d", t.Value),
			})
		case cli.Float64Flag:
			usefulFlags = append(usefulFlags, cli.StringFlag{
				Name:  t.Name,
				Usage: t.Usage,
				Value: fmt.Sprintf("%.2f", t.Value),
			})
		}
	}
	return usefulFlags, nil
}

func writeDoc(templ string, data interface{}, output io.Writer) error {
	funcMap := template.FuncMap{
		"stringifyFlags": stringifyFlags,
		"Prefixed":       prefixedNames,
	}
	tpl := template.Must(template.New("doc").Funcs(funcMap).Parse(templ))
	tabwriter := tabwriter.NewWriter(output, 0, 8, 1, ' ', 0)
	return tpl.Execute(tabwriter, data)
}

// Creates file at correct path. caller must close file.
func createDoc(name string) (*os.File, error) {
	absDoc, err := filepath.Abs(docPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absDoc, 0755); err != nil {
		return nil, err
	}
	tplName := filepath.Join(absDoc,
		fmt.Sprintf("%s.adoc", strings.ToLower(name)))
	return os.Create(tplName)
}

// GenerateDocumentation generates docs for each command
func GenerateDocumentation(options *core.GlobalOptions, app *cli.App) error {

	write := func(name string, templ string, data interface{}) error {
		var w = app.Writer

		if !options.Debug {
			doc, err := createDoc(name)
			if err != nil {
				return err
			}
			defer doc.Close()
			w = doc
		}
		return writeDoc(templ, data, w)
	}
	if err := write("wercker", werckerAppHelpTemplate, app); err != nil {
		return err
	}

	for _, cmd := range app.Commands {
		if err := write(cmd.Name, werckerCommandHelpTemplate, cmd); err != nil {
			return err
		}
	}
	return nil
}
