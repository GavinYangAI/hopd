package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/GavinYangAI/hopd/internal/cli"
	"github.com/GavinYangAI/hopd/internal/config"
	"github.com/GavinYangAI/hopd/internal/daemon"
	"github.com/GavinYangAI/hopd/internal/ipc"
	"github.com/GavinYangAI/hopd/internal/paths"
	"github.com/GavinYangAI/hopd/internal/platform"
	"github.com/GavinYangAI/hopd/internal/tui"
	"github.com/GavinYangAI/hopd/internal/tunnel"
	"github.com/GavinYangAI/hopd/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "hopd:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hopd",
		Short: "Resident SSH port-forwarding daemon",
		RunE:  func(cmd *cobra.Command, args []string) error { return runTUI() }, // Task 14
	}
	root.AddCommand(daemonCmd(), upCmd(), downCmd(), statusCmd(), reloadCmd(), logsCmd(), versionCmd(), installCmd(), uninstallCmd(), authCmd(), tuiCmd())
	return root
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive dashboard (same as running hopd with no command)",
		RunE:  func(cmd *cobra.Command, args []string) error { return runTUI() },
	}
}

func client() *ipc.Client { return ipc.NewClient(paths.SocketPath()) }

func printStatus(resp ipc.Response) error {
	if !resp.OK {
		return fmt.Errorf("%s", resp.Error)
	}
	fmt.Print(cli.FormatStatus(resp.Tunnels))
	return nil
}

func daemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the supervisor in the foreground (used by launchd)",
		RunE: func(cmd *cobra.Command, args []string) error {
			sshPath, err := exec.LookPath("ssh")
			if err != nil {
				return fmt.Errorf("ssh not found in PATH: %w", err)
			}
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return daemon.Run(ctx, paths.ConfigFile(), paths.SocketPath(), paths.ControlDir(), sshPath)
		},
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [name|group|all]",
		Short: "Start tunnels",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Do(ipc.Request{Cmd: ipc.CmdUp, Target: arg0(args, "all")})
			if err != nil {
				return err
			}
			return printStatus(resp)
		},
	}
}

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down [name|group|all]",
		Short: "Stop tunnels",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Do(ipc.Request{Cmd: ipc.CmdDown, Target: arg0(args, "all")})
			if err != nil {
				return err
			}
			return printStatus(resp)
		},
	}
}

func statusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "status",
		Aliases: []string{"ls"},
		Short:   "Show tunnel status",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Do(ipc.Request{Cmd: ipc.CmdStatus})
			if err != nil {
				return err
			}
			return printStatus(resp)
		},
	}
	return c
}

func reloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload config from disk",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Do(ipc.Request{Cmd: ipc.CmdReload})
			if err != nil {
				return err
			}
			return printStatus(resp)
		},
	}
}

func logsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Show captured ssh stderr for a tunnel",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client().Do(ipc.Request{Cmd: ipc.CmdLogs, Target: args[0]})
			if err != nil {
				return err
			}
			if !resp.OK {
				return fmt.Errorf("%s", resp.Error)
			}
			for _, line := range resp.Lines {
				fmt.Println(line)
			}
			return nil
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run:   func(cmd *cobra.Command, args []string) { fmt.Println(version.Version) },
	}
}

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install and start the launchd autostart agent (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			logPath := filepath.Join(paths.ConfigDir(), "hopd.log")
			if err := os.MkdirAll(paths.ConfigDir(), 0o700); err != nil {
				return err
			}
			if err := platform.Install(exe, logPath); err != nil {
				return err
			}
			fmt.Println("hopd installed and started via launchd")
			return nil
		},
	}
}

func authCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auth <name>",
		Short: "Interactively authenticate a tunnel (e.g. 2FA), pre-warming its ssh ControlMaster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(paths.ConfigFile())
			if err != nil {
				return err
			}
			tn, ok := cfg.Tunnel(args[0])
			if !ok {
				return fmt.Errorf("no tunnel named %q", args[0])
			}
			controlDir := paths.ControlDir()
			if err := os.MkdirAll(controlDir, 0o700); err != nil {
				return err
			}
			sshPath, err := exec.LookPath("ssh")
			if err != nil {
				return err
			}
			c := exec.Command(sshPath, tunnel.AuthArgs(tn, controlDir)...)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
			fmt.Println("authenticated; the tunnel will reconnect over the shared connection")
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the launchd autostart agent (macOS)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := platform.Uninstall(); err != nil {
				return err
			}
			fmt.Println("hopd launchd agent removed")
			return nil
		},
	}
}

func arg0(args []string, def string) string {
	if len(args) > 0 {
		return args[0]
	}
	return def
}

func runTUI() error {
	return tui.Run(client())
}
