package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type model struct {
	ctrl    *controller
	dbInfo  map[string]dbInfo
	counter int
}

type dbInfo interface {
	Driver() string
	Name() string
	Conn() *sqlx.DB
	GetTables() ([]string, error)
	GetTableColumns(table string) ([]column, error)
}

func newModel() *model {
	return &model{
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

func (m *model) openDatabase(driver, dsn string) (string, error) {
	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return "", err
	}

	dbID := fmt.Sprintf("%s-%d", driver, m.counter)
	m.counter++

	info, err := m.getDBInfo(driver, dsn, db)
	if err != nil {
		return "", err
	}

	m.dbInfo[dbID] = info

	return dbID, nil
}

func (m *model) getDBInfo(driver, dsn string, db *sqlx.DB) (dbInfo, error) {
	switch driver {
	case "sqlite":
		return &sqliteDbInfo{DSN: dsn, DB: db}, nil
	case "postgres":
		return &pgDbInfo{DSN: dsn, DB: db}, nil
	default:
		return nil, fmt.Errorf("unsupported driver %q", driver)
	}
}

func (m *model) getTables(dbID string) ([]string, error) {
	info := m.dbInfo[dbID]
	if info == nil {
		return nil, fmt.Errorf("database is not open")
	}

	return info.GetTables()
}

func (m *model) getTableColumns(dbID, tbl string) ([]column, error) {
	info := m.dbInfo[dbID]
	if info == nil {
		return nil, fmt.Errorf("database is not open")
	}

	return info.GetTableColumns(tbl)
}

func (m *model) execQuery(dbID, q string) error {
	info := m.dbInfo[dbID]
	if info == nil {
		return fmt.Errorf("database is not open")
	}

	rows, err := info.Conn().Queryx(q)
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
	return info.Name()
}
