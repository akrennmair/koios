package main

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

func (c *controller) getTables() ([]string, error) {
	return c.model.getTables()
}

func (c *controller) getTableColumns(tbl string) ([]column, error) {
	return c.model.getTableColumns(tbl)
}

func (c *controller) execQuery(q string) error {
	return c.model.execQuery(q)
}

func (c *controller) openDatabase(db string) error {
	if err := c.model.openDatabase(db); err != nil {
		return err
	}
	c.view.addDatabase(db)
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
