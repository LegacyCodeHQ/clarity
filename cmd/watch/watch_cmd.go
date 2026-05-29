package watch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/LegacyCodeHQ/clarity/cmd/show/formatters"
	"github.com/LegacyCodeHQ/clarity/internal/mcplogdlog"
	"github.com/spf13/cobra"
)

const maxPortBindAttempts = 20

// Cmd represents the watch command.
var Cmd = NewCommand()

// NewCommand returns a new watch command instance.
func NewCommand() *cobra.Command {
	opts := defaultWatchOptions()

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for file changes and serve a live dependency graph",
		Long:  `Watch a project directory for file changes, rebuild the dependency graph, and serve a live-updating visualization at localhost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWatch(cmd, opts)
		},
	}

	cmd.Flags().StringVarP(&opts.repoPath, "repo", "r", "", "Git repository path (default: current directory)")
	cmd.Flags().IntVarP(&opts.port, "port", "P", opts.port, "HTTP server port")
	cmd.Flags().StringSliceVarP(&opts.includes, "input", "i", nil, "Watch specific files and/or directories (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.excludes, "exclude", nil, "Exclude specific files and/or directories (comma-separated)")
	cmd.Flags().StringVar(&opts.includeExt, "include-ext", "", "Include only files with these extensions (comma-separated, e.g. .go,.java)")
	cmd.Flags().StringVar(&opts.excludeExt, "exclude-ext", "", "Exclude files with these extensions (comma-separated, e.g. .go,.java)")
	cmd.Flags().StringVarP(
		&opts.direction,
		"direction",
		"d",
		opts.direction,
		fmt.Sprintf("Graph direction (%s)", formatters.SupportedDirections()))
	cmd.Flags().BoolVar(&opts.noPhantom, "no-phantom", false, "Suppress phantom test nodes (Rust files with #[cfg(test)] regions are rendered as a single node)")

	return cmd
}

func runWatch(cmd *cobra.Command, opts *watchOptions) error {
	mcplogdlog.Info("watch: start", map[string]any{
		"repo":      opts.repoPath,
		"port":      opts.port,
		"input":     opts.includes,
		"exclude":   opts.excludes,
		"direction": opts.direction,
	})
	repoPath := opts.repoPath
	if repoPath == "" {
		repoPath = "."
	}

	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve repo path: %w", err)
	}
	repoPath = absRepoPath

	if direction, ok := formatters.ParseDirection(opts.direction); !ok {
		return fmt.Errorf("unknown direction: %s (valid options: %s)", opts.direction, formatters.SupportedDirections())
	} else {
		opts.direction = direction.StringLower()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ln, actualPort, err := listenWithPortFallback(opts.port)
	if err != nil {
		mcplogdlog.Error("watch: listen failed", map[string]any{"error": err.Error()})
		return err
	}
	defer ln.Close()

	b := newBroker()
	srv := newServer(b, actualPort, repoPath)
	formatter, err := formatters.NewFormatter("dot")
	if err != nil {
		return err
	}

	go func() {
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			fmt.Fprintf(cmd.ErrOrStderr(), "watch server error: %v\n", serveErr)
		}
	}()

	fmt.Fprintf(cmd.OutOrStdout(), "Watching %s\n", repoPath)
	if actualPort != opts.port {
		fmt.Fprintf(cmd.OutOrStdout(), "Port %d in use, using %d\n", opts.port, actualPort)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Serving at http://localhost:%d\n", actualPort)
	fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl+C to stop\n")

	err = runSupervisor(ctx, repoPath, opts, b, formatter)

	srv.Close()
	return err
}

func listenWithPortFallback(preferredPort int) (net.Listener, int, error) {
	if preferredPort == 0 {
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			return nil, 0, fmt.Errorf("failed to listen on random port: %w", err)
		}
		addr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			return ln, 0, nil
		}
		return ln, addr.Port, nil
	}

	var lastErr error
	for attempt := 0; attempt < maxPortBindAttempts; attempt++ {
		port := preferredPort + attempt
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			return ln, port, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, 0, fmt.Errorf("failed to listen on port %d: %w", port, err)
		}
		lastErr = err
	}

	return nil, 0, fmt.Errorf("failed to listen on ports %d-%d: %w", preferredPort, preferredPort+maxPortBindAttempts-1, lastErr)
}
