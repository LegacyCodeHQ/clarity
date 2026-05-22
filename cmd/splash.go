package cmd

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

const clarityLogo = ` ██████╗██╗      █████╗ ██████╗ ██╗████████╗██╗   ██╗
██╔════╝██║     ██╔══██╗██╔══██╗██║╚══██╔══╝╚██╗ ██╔╝
██║     ██║     ███████║██████╔╝██║   ██║    ╚████╔╝ 
██║     ██║     ██╔══██║██╔══██╗██║   ██║     ╚██╔╝  
╚██████╗███████╗██║  ██║██║  ██║██║   ██║      ██║   
 ╚═════╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝   ╚═╝      ╚═╝   `

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiCyan   = "\x1b[38;5;51m"
	ansiBlue   = "\x1b[38;5;39m"
	ansiPurple = "\x1b[38;5;99m"
	ansiYellow = "\x1b[38;5;226m"
)

func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && cmd.Flags().NFlag() == 0 {
		_, err := fmt.Fprint(cmd.OutOrStdout(), renderSplash(shouldColorSplash()))
		return err
	}

	return cmd.Help()
}

func renderSplash(useColor bool) string {
	logoColors := []string{ansiPurple, ansiPurple, ansiBlue, ansiCyan, ansiCyan, ansiBlue}
	lines := strings.Split(clarityLogo, "\n")
	width := maxLineWidth(lines)

	var b strings.Builder
	b.WriteByte('\n')
	for i, line := range lines {
		color := logoColors[i%len(logoColors)]
		b.WriteString(colorize(line, color+ansiBold, useColor))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(centerText("Software Design Maps for AI-Native Development", width, ansiYellow+ansiBold, useColor))
	b.WriteString("\n\n")
	b.WriteString(centerText("Run 'clarity --help' for usage information", width, ansiDim, useColor))
	b.WriteString("\n\n")

	return b.String()
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		if n := utf8.RuneCountInString(line); n > width {
			width = n
		}
	}
	return width
}

func centerText(text string, width int, color string, useColor bool) string {
	padding := 0
	if textWidth := utf8.RuneCountInString(text); width > textWidth {
		padding = (width - textWidth) / 2
	}

	return strings.Repeat(" ", padding) + colorize(text, color, useColor)
}

func shouldColorSplash() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return !noColor
}

func colorize(text, code string, enabled bool) string {
	if !enabled {
		return text
	}

	return code + text + ansiReset
}
