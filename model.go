package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type model struct {
	ctrl    *controller
	dbs     map[string]*sqlx.DB
	dbInfo  map[string]dbInfo
	counter int
}

type dbInfo struct {
	Driver string
	DSN    string
}

func newModel() *model {
	return &model{
		dbs:    make(map[string]*sqlx.DB),
		dbInfo: make(map[string]dbInfo),
	}
}

func (m *model) setController(c *controller) {
	m.ctrl = c
}

type column struct {
	Name string
	Type string
}

func (m *model) openDatabase(driver, dbName string) (string, error) {
	db, err := sqlx.Open(driver, dbName)
	if err != nil {
		return "", err
	}

	dbID := fmt.Sprintf("%s-%d", driver, m.counter)
	m.counter++

	m.dbs[dbID] = db
	m.dbInfo[dbID] = dbInfo{Driver: driver, DSN: dbName}

	return dbID, nil
}

func (m *model) getTables(dbID string) ([]string, error) {
	if m.dbs[dbID] == nil {
		return nil, fmt.Errorf("database is not open")
	}

	rows, err := m.dbs[dbID].Query("PRAGMA table_list")
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

func (m *model) getTableColumns(dbID, tbl string) ([]column, error) {
	if m.dbs[dbID] == nil {
		return nil, fmt.Errorf("database is not open")
	}

	rows, err := m.dbs[dbID].Query("PRAGMA table_info(" + tbl + ")")
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
			log.Fatalf("Scan failed: %v", err)
		}

		cols = append(cols, column{Name: name, Type: typ})
	}

	return cols, nil
}

func (m *model) execQuery(dbID, q string) error {
	if m.dbs[dbID] == nil {
		return fmt.Errorf("database is not open")
	}

	rows, err := m.dbs[dbID].QueryxContext(context.TODO(), q)
	if err != nil {
		return err
	}
	defer rows.Close()

	m.ctrl.clearResultTable()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	m.ctrl.setResultTableColumns(columns)

	for rows.Next() {
		row, err := rows.SliceScan()
		if err != nil {
			return err
		}
		var values []string
		for _, v := range row {
			values = append(values, fmt.Sprint(v))
		}
		m.ctrl.addResultTableRow(values)
	}
	return nil
}

func (m *model) getDatabaseName(dbID string) string {
	info := m.dbInfo[dbID]
	switch info.Driver {
	case "sqlite":
		return info.DSN
	case "postgres":
		u, _ := url.Parse(info.DSN)
		return fmt.Sprintf("%s/%s", u.Hostname(), u.Path[1:])
	default:
		panic("unsupported driver " + info.Driver)
	}
}
