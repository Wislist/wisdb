package main

import "github.com/spf13/cobra"

func setHelpTemplates(cmd *cobra.Command) {
	cmd.SetHelpTemplate(`{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}{{end}}

{{if or .Runnable .HasSubCommands}}USAGE{{end}}
  {{.UseLine}}

{{if .HasAvailableSubCommands}}COMMANDS{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}

{{if .HasAvailableLocalFlags}}FLAGS
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}

{{if .HasExample}}EXAMPLES
{{.Example}}{{end}}
`)
}
