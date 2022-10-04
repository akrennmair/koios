package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/jmoiron/sqlx"
	"github.com/rivo/tview"
)

var supportedDrivers = map[string]struct {
	Name             string
	DSNGenerator     func(params connectParams) string
	DBInfoGenerator  func(params connectParams, db *sqlx.DB) dbInfo
	AddInputFields   func(form *tview.Form)
	GetConnectParams func(form *tview.Form) connectParams
}{
	"sqlite": {
		Name: "SQLite",
		DSNGenerator: func(params connectParams) string {
			return params["file"]
		},
		DBInfoGenerator: func(params connectParams, db *sqlx.DB) dbInfo {
			return &sqliteDbInfo{Params: params, DB: db}
		},
		AddInputFields: func(form *tview.Form) {
			form.AddInputField("Filename", "", 30, nil, nil)
		},
		GetConnectParams: func(form *tview.Form) connectParams {
			file := form.GetFormItem(0).(*tview.InputField).GetText()
			abspath, err := filepath.Abs(file)
			if err != nil {
				log.Printf("filepath.Abs %s failed: %v", file, err)
				abspath = file
			}
			return connectParams{"file": abspath}
		},
	},
	"postgres": {
		Name: "PostgreSQL",
		DSNGenerator: func(params connectParams) string {
			return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
				params["user"], params["password"], params["host"], params["port"], params["db"], params["ssl_mode"])
		},
		DBInfoGenerator: func(params connectParams, db *sqlx.DB) dbInfo {
			return &pgDbInfo{Params: params, DB: db}
		},
		AddInputFields: func(form *tview.Form) {
			form.
				AddInputField("Database", "", 30, nil, nil).
				AddInputField("User", "", 30, nil, nil).
				AddPasswordField("Password", "", 30, '*', nil).
				AddInputField("Host", "localhost", 30, nil, nil).
				AddInputField("Port", "5432", 30, func(textToCheck string, lastChar rune) bool {
					i, err := strconv.ParseUint(textToCheck, 10, 64)
					return err == nil && i >= 1 && i <= 65535
				}, nil).
				AddDropDown("SSL Mode", []string{"disable", "require", "verify-ca", "verify-full"}, 0, nil)
		},
		GetConnectParams: func(form *tview.Form) connectParams {
			db := form.GetFormItem(0).(*tview.InputField).GetText()
			user := form.GetFormItem(1).(*tview.InputField).GetText()
			password := form.GetFormItem(2).(*tview.InputField).GetText()
			host := form.GetFormItem(3).(*tview.InputField).GetText()
			port := form.GetFormItem(4).(*tview.InputField).GetText()
			_, sslMode := form.GetFormItem(5).(*tview.DropDown).GetCurrentOption()
			return connectParams{
				"db":       db,
				"user":     user,
				"password": password,
				"host":     host,
				"port":     port,
				"ssl_mode": sslMode,
			}
		},
	},
	"athena": {
		Name: "Athena",
		DSNGenerator: func(params connectParams) string {
			values := new(url.Values)
			for k, v := range params {
				values.Set(k, v)
			}
			return values.Encode()
		},
		DBInfoGenerator: func(params connectParams, db *sqlx.DB) dbInfo {
			return &athenaDbInfo{Params: params, DB: db}
		},
		AddInputFields: func(form *tview.Form) {
			form.
				AddInputField("Database", "", 30, nil, nil).
				AddInputField("Output Location", "", 30, nil, nil).
				AddInputField("Workgroup", "primary", 30, nil, nil).
				AddInputField("AWS Access Key ID", "", 30, nil, nil).
				AddPasswordField("AWS Secret Access Key", "", 30, '*', nil).
				AddInputField("AWS Region", "", 30, nil, nil)
		},
		GetConnectParams: func(form *tview.Form) connectParams {
			db := form.GetFormItem(0).(*tview.InputField).GetText()
			outputLocation := form.GetFormItem(1).(*tview.InputField).GetText()
			workgroup := form.GetFormItem(2).(*tview.InputField).GetText()
			awsAccessKeyID := form.GetFormItem(3).(*tview.InputField).GetText()
			awsSecretAccessKey := form.GetFormItem(4).(*tview.InputField).GetText()
			awsRegion := form.GetFormItem(5).(*tview.InputField).GetText()
			return connectParams{
				"db":                db,
				"output_location":   outputLocation,
				"workgroup":         workgroup,
				"access_key_id":     awsAccessKeyID,
				"secret_access_key": awsSecretAccessKey,
				"region":            awsRegion,
			}
		},
	},
}

func supportedDriverList() []string {
	var drivers []string
	for k := range supportedDrivers {
		drivers = append(drivers, k)
	}
	sort.Strings(drivers)
	return drivers
}

type dbInfo interface {
	Driver() string
	ConnectParams() connectParams
	Name() string
	Conn() *sqlx.DB
	GetTables() ([]string, error)
	GetTableColumns(table string) ([]column, error)
}

type sqliteDbInfo struct {
	Params connectParams
	DB     *sqlx.DB
}

func (i *sqliteDbInfo) Driver() string {
	return "sqlite"
}

func (i *sqliteDbInfo) ConnectParams() connectParams {
	return i.Params
}

func (i *sqliteDbInfo) Name() string {
	return filepath.Base(i.Params["file"])
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
	Params connectParams
	DB     *sqlx.DB
}

func (i *pgDbInfo) Driver() string {
	return "postgres"
}

func (i *pgDbInfo) ConnectParams() connectParams {
	return i.Params
}

func (i *pgDbInfo) Name() string {
	return fmt.Sprintf("%s/%s", i.Params["host"], i.Params["db"])
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

type athenaDbInfo struct {
	Params connectParams
	DB     *sqlx.DB
}

func (i *athenaDbInfo) Driver() string {
	return "athena"
}

func (i *athenaDbInfo) ConnectParams() connectParams {
	return i.Params
}

func (i *athenaDbInfo) Name() string {
	name := i.Params["db"]
	if name == "" {
		name = "Athena"
	}
	return name
}

func (i *athenaDbInfo) Conn() *sqlx.DB {
	return i.DB
}

func (i *athenaDbInfo) GetTables() ([]string, error) {
	name := i.Params["db"]
	rows, err := i.DB.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = '" + name + "'")
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

func (i *athenaDbInfo) GetTableColumns(table string) ([]column, error) {
	name := i.Params["db"]
	rows, err := i.DB.Query("select column_name, data_type from information_schema.columns where table_schema = '" + name + "' and table_name = '" + table + "'")
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
