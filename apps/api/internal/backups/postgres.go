package backups

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

const resetSchema = "DROP SCHEMA public CASCADE;\nCREATE SCHEMA public;\n"

// connEnv passes credentials through the environment so they never appear in process arguments.
func (s *Service) connEnv(database string) ([]string, error) {
	u, err := url.Parse(s.dsn)
	if err != nil {
		return nil, errors.New("invalid database url")
	}
	password, _ := u.User.Password()
	if database == "" {
		database = strings.TrimPrefix(u.Path, "/")
	}
	env := append(os.Environ(),
		"PGHOST="+u.Hostname(),
		"PGPORT="+u.Port(),
		"PGUSER="+u.User.Username(),
		"PGPASSWORD="+password,
		"PGDATABASE="+database,
	)
	if mode := u.Query().Get("sslmode"); mode != "" {
		env = append(env, "PGSSLMODE="+mode)
	}
	return env, nil
}

// pgDump leaves out the rows of backups: that history describes the files on this server,
// not the moment being saved, so a restore must not rewind it.
func (s *Service) pgDump(ctx context.Context, file string) error {
	env, err := s.connEnv("")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-privileges",
		"--exclude-table-data=public.backups", "--file="+file)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump: %w: %s", err, tail(stderr.String()))
	}
	return nil
}

// load applies the archive in a single transaction, so any failure leaves the target database
// as it was; the prelude runs inside that transaction. The full script is written first:
// piping it would make psql commit a partial load if pg_restore died halfway.
func (s *Service) load(ctx context.Context, file, database, prelude string) error {
	env, err := s.connEnv(database)
	if err != nil {
		return err
	}
	script, err := os.CreateTemp(s.dir, "restore-*.sql")
	if err != nil {
		return err
	}
	defer os.Remove(script.Name())
	defer script.Close()

	extract := exec.CommandContext(ctx, "pg_restore", "--no-owner", "--no-privileges", "--file="+script.Name(), file)
	var stderr bytes.Buffer
	extract.Stderr = &stderr
	if err := extract.Run(); err != nil {
		return fmt.Errorf("pg_restore: %w: %s", err, tail(stderr.String()))
	}

	psql := exec.CommandContext(ctx, "psql", "--no-psqlrc", "--quiet", "--single-transaction",
		"--set=ON_ERROR_STOP=1", "--output=/dev/null", "--file=-")
	psql.Env = env
	psql.Stdin = io.MultiReader(strings.NewReader(prelude), script)
	stderr.Reset()
	psql.Stderr = &stderr
	if err := psql.Run(); err != nil {
		return fmt.Errorf("psql: %w: %s", err, tail(stderr.String()))
	}
	return nil
}

func tail(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 500 {
		return text[len(text)-500:]
	}
	return text
}
