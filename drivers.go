package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

type sqliteDbInfo struct {
	DSN string
	DB  *sqlx.DB
}

func (i *sqliteDbInfo) Driver() string {
	return "sqlite"
}

func (i *sqliteDbInfo) Name() string {
	return filepath.Base(i.DSN)
}

func (i *sqliteDbInfo) Conn() *sqlx.DB {
	return i.DB
}

func (i *sqliteDbInfo) GetTables() ([]string, error) {
	rows, err := i.DB.Query("PRAGMA table_list")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string

	for rows.Next() {
		var (
			schema, name, typ string
			ncol              int
			wr                int
			strict            bool
		)
		if err := rows.Scan(&schema, &name, &typ, &ncol, &wr, &strict); err != nil {
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		tables = append(tables, name)
	}

	return tables, nil
}

func (i *sqliteDbInfo) GetTableColumns(tbl string) ([]column, error) {
	rows, err := i.DB.Query("PRAGMA table_info(" + tbl + ")")
	if err != nil {
		return nil, fmt.Errorf("querying columns for %s failed: %v", tbl, err)

	}
	defer rows.Close()

	var cols []column

	for rows.Next() {
		var (
			cid       int
			name, typ string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)

		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}

		cols = append(cols, column{Name: name, Type: typ})
	}

	return cols, nil
}

type pgDbInfo struct {
	DSN string
	DB  *sqlx.DB
}

func (i *pgDbInfo) Driver() string {
	return "postgres"
}

func (i *pgDbInfo) Name() string {
	u, _ := url.Parse(i.DSN)
	return fmt.Sprintf("%s%s", u.Hostname(), u.Path)
}

func (i *pgDbInfo) Conn() *sqlx.DB {
	return i.DB
}

func (i *pgDbInfo) GetTables() ([]string, error) {
	rows, err := i.DB.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}

func (i *pgDbInfo) GetTableColumns(tbl string) ([]column, error) {
	rows, err := i.DB.Query("select column_name, data_type from information_schema.columns where table_schema = 'public' and table_name = $1", tbl)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []column
	for rows.Next() {
		var (
			col string
			typ string
		)
		if err := rows.Scan(&col, &typ); err != nil {
			return nil, err
		}
		cols = append(cols, column{Name: col, Type: typ})
	}

	return cols, nil
}
