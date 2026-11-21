package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

type securityQuery struct {
	text string
	args []driver.NamedValue
}

type securityDriver struct{ calls *[]securityQuery }
type securityConn struct{ calls *[]securityQuery }
type securityTx struct{}
type securityRows struct {
	columns []string
	values  []driver.Value
	done    bool
}

var securityDriverNumber atomic.Uint64

func (d securityDriver) Open(string) (driver.Conn, error) { return securityConn{calls: d.calls}, nil }
func (c securityConn) Close() error                       { return nil }
func (c securityConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("unexpected Prepare")
}
func (c securityConn) Begin() (driver.Tx, error) { return securityTx{}, nil }
func (securityTx) Commit() error                 { return nil }
func (securityTx) Rollback() error               { return nil }
func (r *securityRows) Columns() []string        { return r.columns }
func (r *securityRows) Close() error             { return nil }
func (r *securityRows) Next(values []driver.Value) error {
	if r.done || r.values == nil {
		return io.EOF
	}
	copy(values, r.values)
	r.done = true
	return nil
}

func (c securityConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	*c.calls = append(*c.calls, securityQuery{text: query, args: append([]driver.NamedValue(nil), args...)})
	return driver.RowsAffected(1), nil
}

func (c securityConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	*c.calls = append(*c.calls, securityQuery{text: query, args: append([]driver.NamedValue(nil), args...)})
	if strings.Contains(query, "SELECT id,username,password_hash") {
		return &securityRows{columns: []string{"id", "username", "password_hash"}, values: []driver.Value{int64(1), args[0].Value, "hash"}}, nil
	}
	if strings.Contains(query, "SELECT id,timestamp") {
		return &securityRows{columns: []string{"id", "timestamp", "type", "path", "status", "error"}}, nil
	}
	return &securityRows{columns: []string{"id"}, values: []driver.Value{int64(1)}}, nil
}

func TestSecuritySQLValuesAreNotQueryText(t *testing.T) {
	var calls []securityQuery
	name := fmt.Sprintf("kfm_security_%d", securityDriverNumber.Add(1))
	sql.Register(name, securityDriver{calls: &calls})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	store := &Store{DB: db}
	login := `x'; DROP TABLE users; --`
	path := `note'; DROP TABLE files; --`
	command := `create'); DELETE FROM operations; --`
	message := `failure'; DELETE FROM users; --`
	user, err := store.CreateUser(login, "stored hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.GetUser(login); err != nil {
		t.Fatal(err)
	}
	id, err := store.Start(user.ID, command, path)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.FinishFiles(id, user.ID, []File{{Path: path, Size: 1}}, fmt.Errorf("%s", message)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.History(user.ID); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, call := range calls {
		if !strings.Contains(call.text, "$1") {
			t.Errorf("query is not parameterized: %s", call.text)
		}
		for _, payload := range []string{login, path, command, message} {
			if strings.Contains(call.text, payload) {
				t.Errorf("user value became SQL: %s", payload)
			}
			for _, arg := range call.args {
				if arg.Value == payload {
					found[payload] = true
				}
			}
		}
	}
	for _, payload := range []string{login, path, command, message} {
		if !found[payload] {
			t.Errorf("did not exercise parameter: %s", payload)
		}
	}
}
