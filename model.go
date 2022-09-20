package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

type model struct {
	ctrl *controller
	db   *sqlx.DB
}

func newModel() *model {
	return &model{}
}

func (m *model) setController(c *controller) {
	m.ctrl = c
}

type column struct {
	Name string
	Type string
}

func (m *model) openDatabase(dbName string) error {
	db, err := sqlx.Open("sqlite", dbName)
	if err != nil {
		return err
	}

	m.db = db
	return nil
}

func (m *model) getTables() ([]string, error) {
	if m.db == nil {
		return nil, fmt.Errorf("no database is open")
	}

	rows, err := m.db.Query("PRAGMA table_list")
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

func (m *model) getTableColumns(tbl string) ([]column, error) {
	if m.db == nil {
		return nil, fmt.Errorf("no database is open")
	}

	rows, err := m.db.Query("PRAGMA table_info(" + tbl + ")")
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

func (m *model) execQuery(q string) error {
	if m.db == nil {
		return fmt.Errorf("no database is open")
	}

	rows, err := m.db.QueryxContext(context.TODO(), q)
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
