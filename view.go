package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
	"github.com/rivo/tview"
)

type mainView struct {
	ctrl *controller

	app          *tview.Application
	layout       *tview.Flex
	dbTree       *tview.TreeView
	queryInput   *tview.TextArea
	resultTable  *tview.Table
	infoLine     *tview.Flex
	contextField *tview.TextView

	activityGauge       *tvxwidgets.ActivityModeGauge
	activityPlaceholder *tview.Box
	gaugeC              chan struct{}

	dbRootNode *tview.TreeNode

	currentResultTableRow int
	currentDB             string // currently selected dbID

	keyMapping       map[string]string // mapping of key to operation
	operationMapping map[string]func() // mapping of operation to function
}

type nodeRef struct {
	Type  nodeType
	DB    string
	Table string
}

type nodeType int

const (
	typeDB nodeType = iota
	typeTable
)

func newMainView() *mainView {
	view := &mainView{
		gaugeC:           make(chan struct{}, 1),
		keyMapping:       make(map[string]string),
		operationMapping: make(map[string]func()),
	}
	view.setup()

	return view
}

func (v *mainView) configure(cfg config) error {
	v.operationMapping["quit"] = v.quit
	v.operationMapping["goto-queryinput"] = v.gotoQueryInput
	v.operationMapping["goto-tree"] = v.gotoTree
	v.operationMapping["goto-result"] = v.gotoResultTable
	v.operationMapping["set-current-db"] = v.setCurrentDatabase
	v.operationMapping["add-db"] = v.addDatabaseDialog
	v.operationMapping["exec-query"] = v.execQuery

	v.keyMapping["Ctrl+Q"] = "quit"
	v.keyMapping["Tab"] = "goto-queryinput"
	v.keyMapping["Ctrl+T"] = "goto-tree"
	v.keyMapping["Ctrl+R"] = "goto-result"
	v.keyMapping["Ctrl+S"] = "set-current-db"
	v.keyMapping["Ctrl+A"] = "add-db"
	v.keyMapping["Ctrl+Space"] = "exec-query"

	for _, keyCfg := range cfg.Keys {
		v.keyMapping[keyCfg.Key] = keyCfg.Operation
	}

	for _, op := range v.keyMapping {
		if _, ok := v.operationMapping[op]; !ok {
			return fmt.Errorf("unknown operation %q", op)
		}
	}

	return nil
}

func (v *mainView) setController(c *controller) {
	v.ctrl = c
}

func (v *mainView) setup() {
	v.app = tview.NewApplication()

	v.dbRootNode = tview.NewTreeNode("Databases")

	v.dbTree = tview.NewTreeView()
	v.dbTree.SetBorder(true).SetTitle("Databases")
	v.dbTree.SetRoot(v.dbRootNode).SetCurrentNode(v.dbRootNode)
	v.dbTree.SetSelectedFunc(v.treeNodeSelected)

	v.queryInput = tview.NewTextArea()
	v.queryInput.SetBorder(true).SetTitle("Query")

	v.resultTable = tview.NewTable()
	v.resultTable.SetBorder(true).SetTitle("Result")
	v.resultTable.SetBorders(true)

	v.contextField = tview.NewTextView()
	v.activityGauge = tvxwidgets.NewActivityModeGauge()
	v.activityPlaceholder = tview.NewBox()

	v.infoLine = tview.NewFlex().
		AddItem(v.contextField, 0, 1, false).
		AddItem(v.activityPlaceholder, 0, 3, false)

	v.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(v.dbTree, 0, 1, true).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(v.queryInput, 0, 1, false).
				AddItem(v.resultTable, 0, 3, false), 0, 3, false), 0, 1, false).
		AddItem(v.infoLine, 1, 1, false)

	v.layout.SetInputCapture(v.handleKey)

	v.app.EnableMouse(true)

	v.showMainView()
}

func (v *mainView) startActivityGauge() {
	log.Printf("Starting activity gauge")

	v.activityGauge.Reset()
	v.infoLine.RemoveItem(v.activityPlaceholder)
	v.infoLine.AddItem(v.activityGauge, 0, 3, false)

	ticker := time.NewTicker(250 * time.Millisecond)

	go func() {
		defer func() {
			ticker.Stop()
			v.app.QueueUpdateDraw(func() {
				v.infoLine.RemoveItem(v.activityGauge)
				v.infoLine.AddItem(v.activityPlaceholder, 0, 3, false)
				log.Printf("Stopped activity gauge")
			})
		}()

		for {
			select {
			case <-ticker.C:
				log.Printf("Activity gauge pulse")
				v.app.QueueUpdateDraw(func() {
					v.activityGauge.Pulse()
				})
				log.Printf("After activity gauge pulse draw")
			case <-v.gaugeC:
				return
			}
		}
	}()
}

func (v *mainView) stopActivityGauge() {
	v.gaugeC <- struct{}{}
}

func (v *mainView) showMainView() {
	v.app.SetRoot(v.layout, true)
	v.app.SetFocus(v.dbTree)
}

func (v *mainView) treeNodeSelected(node *tview.TreeNode) {
	ref, ok := node.GetReference().(*nodeRef)
	if !ok {
		node.SetExpanded(!node.IsExpanded())

		return
	}

	if len(node.GetChildren()) > 0 {
		node.SetExpanded(!node.IsExpanded())

		return
	}

	switch ref.Type {
	case typeDB:
		go func() {
			v.startActivityGauge()
			defer v.stopActivityGauge()

			log.Printf("Getting list of tables from database %s", ref.DB)

			tables, err := v.ctrl.getTables(ref.DB)
			if err != nil {
				v.showError("Listing tables failed: %v", err)

				return
			}

			for _, table := range tables {
				tblNode := tview.NewTreeNode(table).
					SetSelectable(true).
					SetReference(&nodeRef{Type: typeTable, DB: ref.DB, Table: table})
				node.AddChild(tblNode)
			}
			log.Printf("Finished getting list of tables from database %s", ref.DB)
		}()
	case typeTable:
		go func() {
			v.startActivityGauge()
			defer v.stopActivityGauge()

			fields, err := v.ctrl.getTableColumns(ref.DB, ref.Table)
			if err != nil {
				v.showError("Listing columns for %s failed: %v", ref.Table, err)
			}

			for _, field := range fields {
				node.AddChild(tview.NewTreeNode(field.Name + " (" + field.Type + ")"))
			}
		}()
	}
}

func (v *mainView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	keyName := event.Name()

	log.Printf("Handling key %s", keyName)

	op, ok := v.keyMapping[keyName]
	if !ok {
		log.Printf("No key mapping found for key %s", keyName)
		return event
	}

	opFunc, ok := v.operationMapping[op]
	if !ok {
		log.Printf("Operation %s not found", op)
		return event
	}

	log.Printf("Received key %s and executing operation %s", keyName, op)

	opFunc()

	return nil
}

func (v *mainView) quit() {
	v.app.Stop()
}

func (v *mainView) gotoQueryInput() {
	v.app.SetFocus(v.queryInput)
}

func (v *mainView) gotoTree() {
	v.app.SetFocus(v.dbTree)
}

func (v *mainView) gotoResultTable() {
	v.app.SetFocus(v.resultTable)
}

func (v *mainView) setCurrentDatabase() {
	currentNode := v.dbTree.GetCurrentNode()
	if currentNode != nil {
		ref, ok := currentNode.GetReference().(*nodeRef)
		if ok {
			if ref.Type == typeDB {
				v.setCurrentDB(ref.DB)
			}
		}
	}
}

func (v *mainView) execQuery() {
	if v.currentDB == "" {
		v.showError("No database has been selected")

		return
	}

	go func() {
		v.startActivityGauge()
		defer v.stopActivityGauge()

		if err := v.ctrl.execQuery(v.currentDB, v.queryInput.GetText()); err != nil {
			v.showError("Query failed: %v", err)

			return
		}

		v.app.SetFocus(v.resultTable)
	}()
}

func (v *mainView) addDatabaseDialog() {
	selectedOption := ""
	form := tview.NewForm().AddDropDown("Driver", supportedDriverList(), 0, func(option string, optionIndex int) {
		selectedOption = option
	}).AddButton("Next", func() {
		v.dbParamsDialog(selectedOption)
	}).AddButton("Cancel", func() {
		v.showMainView()
	})

	form.SetBorder(true).SetTitle("Add Database - Choose Driver")

	v.app.SetRoot(form, true)
}

func (v *mainView) dbParamsDialog(driver string) {
	drv := supportedDrivers[driver]
	form := tview.NewForm()
	drv.AddInputFields(form)
	form.AddButton("Add Database", func() {
		params := drv.GetConnectParams(form)
		if err := v.ctrl.openDatabase(driver, params); err != nil {
			log.Printf("Opening database %s %+v failed: %v", driver, params, err)
		}
		v.showMainView()
	}).AddButton("Cancel", func() {
		v.showMainView()
	})
	form.SetBorder(true).SetTitle(fmt.Sprintf("Add Database - %s Configuration", drv.Name))
	v.app.SetRoot(form, true)
}

func (v *mainView) addDatabase(dbID, dbName string) {
	v.dbRootNode.AddChild(tview.NewTreeNode(dbName).SetSelectable(true).SetReference(&nodeRef{Type: typeDB, DB: dbID}))

	if v.currentDB == "" { // if no database been selected yet, simply set it to database that is being added.
		v.setCurrentDB(dbID)
	}
}

func (v *mainView) setCurrentDB(dbID string) {
	v.currentDB = dbID
	dbName := v.ctrl.getDatabaseName(dbID)
	v.contextField.SetText("Current DB: " + dbName)
}

func (v *mainView) clearResultTable() {
	v.resultTable.Clear()
	v.currentResultTableRow = 1
}

func (v *mainView) setResultTableColumns(cols []string) {
	for idx, col := range cols {
		v.resultTable.SetCell(0, idx, tview.NewTableCell(col).SetAttributes(tcell.AttrBold))
	}
}

func (v *mainView) addResultTableRow(values []string) {
	for idx, val := range values {
		v.resultTable.SetCell(v.currentResultTableRow, idx, tview.NewTableCell(val))
	}
	v.currentResultTableRow++
}

func (v *mainView) showError(s string, args ...any) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf(s, args...)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			v.showMainView()
		})

	v.app.SetRoot(modal, false)
}

func (v *mainView) run() error {
	if err := v.app.Run(); err != nil {
		return fmt.Errorf("running application failed: %w", err)
	}

	return nil
}
