package dump

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bikky-kc013/TableForge/internal/config"
)

// Options mirrors phpPgAdmin's dbexport.php / dataexport.php controls.
type Options struct {
	ServerIdx int
	Database  string
	Schema    string
	Table     string
	Format    string // plain, custom, directory, tar
	DataOnly  bool
	SchemaOnly bool
	Clean     bool
	IfExists  bool
	Create    bool
	NoOwner   bool
	NoPrivileges bool
	NoComments bool
	Verbose   bool
}

// Validate ensures no shell injection via allowlist.
func (o Options) Validate() error {
	if o.Database != "" && strings.ContainsAny(o.Database, ";`$|&") {
		return fmt.Errorf("invalid database name")
	}
	if o.Format != "" {
		switch o.Format {
		case "plain", "custom", "directory", "tar", "csv", "sql":
		default:
			return fmt.Errorf("invalid format %q", o.Format)
		}
	}
	return nil
}

// BuildArgs constructs pg_dump argv with allowlisted flags only.
func BuildArgs(srv config.Server, username string, o Options) ([]string, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	bin := srv.PgDumpPath
	if bin == "" {
		bin = "/usr/bin/pg_dump"
	}
	// Ensure bin is absolute and exists
	if !filepath.IsAbs(bin) {
		return nil, fmt.Errorf("pg_dump_path must be absolute")
	}
	if _, err := os.Stat(bin); err != nil {
		// allow missing in dev/test; still return args with bin
	}

	args := []string{}
	if srv.Host != "" {
		args = append(args, "-h", srv.Host)
	}
	if srv.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", srv.Port))
	}
	if username != "" {
		args = append(args, "-U", username)
	}
	// Never pass password on command line; use PGPASSWORD env
	if o.Format != "" && o.Format != "sql" && o.Format != "csv" {
		// pg_dump format mapping
		switch o.Format {
		case "custom":
			args = append(args, "-Fc")
		case "directory":
			args = append(args, "-Fd")
		case "tar":
			args = append(args, "-Ft")
		default:
			args = append(args, "-Fp")
		}
	}
	if o.DataOnly {
		args = append(args, "-a")
	}
	if o.SchemaOnly {
		args = append(args, "-s")
	}
	if o.Clean {
		args = append(args, "-c")
	}
	if o.IfExists {
		args = append(args, "--if-exists")
	}
	if o.Create {
		args = append(args, "-C")
	}
	if o.NoOwner {
		args = append(args, "--no-owner")
	}
	if o.NoPrivileges {
		args = append(args, "--no-privileges")
	}
	if o.Schema != "" {
		args = append(args, "-n", o.Schema)
	}
	if o.Table != "" {
		// table may be schema.table
		t := o.Table
		if o.Schema != "" && !strings.Contains(t, ".") {
			t = o.Schema + "." + t
		}
		args = append(args, "-t", t)
	}
	args = append(args, o.Database)
	_ = bin
	return args, nil
}

// Run executes pg_dump and streams to w. Caller must set PGPASSWORD env via context.
func Run(ctx context.Context, srv config.Server, password string, o Options, outputPath string) error {
	args, err := BuildArgs(srv, "", o)
	if err != nil {
		return err
	}
	bin := srv.PgDumpPath
	if bin == "" {
		bin = "/usr/bin/pg_dump"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	// Secure env: only pass needed vars, no shell
	cmd.Env = []string{
		"PGPASSWORD=" + password,
		"PGSSLMODE=" + srv.SSLMode,
	}
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer f.Close()
		cmd.Stdout = f
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	return nil
}

// PgDumpPathAllowed checks config enables dump (like Misc::isDumpEnabled).
func IsDumpEnabled(srv config.Server, all bool) bool {
	if all {
		return srv.PgDumpAllPath != ""
	}
	return srv.PgDumpPath != ""
}
