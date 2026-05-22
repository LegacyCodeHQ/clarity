package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
		out := cmd.OutOrStdout()
		_, err := fmt.Fprint(out, renderSplash(terminalWidth(out), shouldColorSplash()))
		return err
	}

	return cmd.Help()
}

func renderSplash(screenWidth int, useColor bool) string {
	logoColors := []string{ansiPurple, ansiPurple, ansiBlue, ansiCyan, ansiCyan, ansiBlue}
	lines := strings.Split(clarityLogo, "\n")
	width := maxLineWidth(lines)
	outerPadding := centerPadding(width, screenWidth)
	outerIndent := strings.Repeat(" ", outerPadding)

	var b strings.Builder
	b.WriteByte('\n')
	for i, line := range lines {
		color := logoColors[i%len(logoColors)]
		b.WriteString(outerIndent)
		b.WriteString(colorize(line, color+ansiBold, useColor))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(outerIndent)
	b.WriteString(centerText("Software Design Maps for AI-Native Development", width, ansiYellow+ansiBold, useColor))
	b.WriteString("\n\n")
	b.WriteString(outerIndent)
	b.WriteString(centerText("Run 'clarity --help' for usage information", width, ansiDim, useColor))
	b.WriteString("\n\n")

	return b.String()
}

func terminalWidth(out io.Writer) int {
	if columns := os.Getenv("COLUMNS"); columns != "" {
		if width, err := strconv.Atoi(columns); err == nil && width > 0 {
			return width
		}
	}

	file, ok := out.(*os.File)
	if !ok {
		return 0
	}

	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0
	}

	return width
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

func centerPadding(contentWidth, screenWidth int) int {
	if screenWidth <= contentWidth {
		return 0
	}

	return (screenWidth - contentWidth) / 2
}

func centerText(text string, width int, color string, useColor bool) string {
	return strings.Repeat(" ", centerPadding(utf8.RuneCountInString(text), width)) + colorize(text, color, useColor)
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
