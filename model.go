package main

import (
	"errors"
	"fmt"

	_ "github.com/akrennmair/go-athena"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type model struct {
	ctrl    *controller
	dbInfo  map[string]dbInfo
	counter int
}

type sessionData struct {
	Databases []sessionDataDB `yaml:"databases"`
}

type sessionDataDB struct {
	Driver        string            `yaml:"driver"`
	ConnectParams map[string]string `yaml:"connect_params"`
}

type connectParams map[string]string

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

var (
	errDatabaseNotOpen   = errors.New("database is not open")
	errUnsupportedDriver = errors.New("unsupported driver")
)

func (m *model) openDatabase(driver string, params connectParams) (string, error) {
	dsn, err := m.getDSN(driver, params)
	if err != nil {
		return "", err
	}

	dbConn, err := sqlx.Open(driver, dsn)
	if err != nil {
		return "", fmt.Errorf("opening %s database failed: %w", driver, err)
	}

	dbID := fmt.Sprintf("%s-%d", driver, m.counter)
	m.counter++

	info, err := m.getDBInfo(driver, params, dbConn)
	if err != nil {
		return "", err
	}

	m.dbInfo[dbID] = info

	return dbID, nil
}

func (m *model) getDSN(driver string, params connectParams) (string, error) {
	drv, ok := supportedDrivers[driver]
	if !ok {
		return "", fmt.Errorf("%s: %w", driver, errUnsupportedDriver)
	}

	return drv.DSNGenerator(params), nil
}

func (m *model) getDBInfo(driver string, params connectParams, dbConn *sqlx.DB) (dbInfo, error) {
	drv, ok := supportedDrivers[driver]
	if !ok {
		return nil, fmt.Errorf("%s: %w", driver, errUnsupportedDriver)
	}

	return drv.DBInfoGenerator(params, dbConn), nil
}

func (m *model) getTables(dbID string) ([]string, error) {
	info := m.dbInfo[dbID]
	if info == nil {
		return nil, errDatabaseNotOpen
	}

	return info.GetTables()
}

func (m *model) getTableColumns(dbID, tbl string) ([]column, error) {
	info := m.dbInfo[dbID]
	if info == nil {
		return nil, errDatabaseNotOpen
	}

	return info.GetTableColumns(tbl)
}

func (m *model) execQuery(dbID, query string) error {
	info := m.dbInfo[dbID]
	if info == nil {
		return errDatabaseNotOpen
	}

	rows, err := info.Conn().Queryx(query)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	m.ctrl.clearResultTable()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("listing columns failed: %w", err)
	}

	m.ctrl.setResultTableColumns(columns)

	for rows.Next() {
		row, err := rows.SliceScan()
		if err != nil {
			return fmt.Errorf("scanning row failed: %w", err)
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
	return m.dbInfo[dbID].Name()
}

func (m *model) getSession() *sessionData {
	var session sessionData
	for _, dbInfo := range m.dbInfo {
		session.Databases = append(session.Databases, sessionDataDB{
			Driver:        dbInfo.Driver(),
			ConnectParams: dbInfo.ConnectParams(),
		})
	}

	return &session
}
