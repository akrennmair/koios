package main

import "log"

type controller struct {
	model *model
	view  *mainView
}

func newController(model *model, view *mainView) *controller {
	return &controller{
		model: model,
		view:  view,
	}
}

func (c *controller) getTables(dbID string) ([]string, error) {
	return c.model.getTables(dbID)
}

func (c *controller) getTableColumns(dbID, tbl string) ([]column, error) {
	return c.model.getTableColumns(dbID, tbl)
}

func (c *controller) execQuery(dbID, q string) error {
	return c.model.execQuery(dbID, q)
}

func (c *controller) openDatabase(driver string, params connectParams) error {
	dbID, err := c.model.openDatabase(driver, params)
	if err != nil {
		return err
	}

	c.view.addDatabase(dbID, c.model.getDatabaseName(dbID))

	return nil
}

func (c *controller) clearResultTable() {
	c.view.clearResultTable()
}

func (c *controller) setResultTableColumns(columns []string) {
	c.view.setResultTableColumns(columns)
}

func (c *controller) addResultTableRow(values []string) {
	c.view.addResultTableRow(values)
}

func (c *controller) getSession() *sessionData {
	session := &sessionData{
		Databases: c.model.getSession(),
		Queries:   c.view.getSession(),
	}
	return session
}

func (c *controller) restoreSession(session *sessionData) {
	for _, db := range session.Databases {
		if err := c.openDatabase(db.Driver, db.ConnectParams); err != nil {
			log.Printf("Opening database %s %+v failed: %v", db.Driver, db.ConnectParams, err)
		}
	}

	c.view.restoreSession(session.Queries)
}

func (c *controller) getDatabaseName(dbID string) string {
	return c.model.getDatabaseName(dbID)
}

func (c *controller) closeDatabase(dbID string) string {
	return c.model.closeDatabase(dbID)
}
