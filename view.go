package main

import (
	"fmt"
	"log"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type mainView struct {
	ctrl *controller

	app         *tview.Application
	layout      *tview.Flex
	dbTree      *tview.TreeView
	queryInput  *tview.TextArea
	resultTable *tview.Table
	infoLine    *tview.TextView

	dbRootNode *tview.TreeNode

	currentResultTableRow int
	currentDB             string // currently selected dbID
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
	view := &mainView{}
	view.setup()

	return view
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

	infoText := "^Q Quit | ^I Query Input | ^T DB Tree | ^R Result | ^S Select DB | ^Space Run Query | ^A Add DB"

	v.infoLine = tview.NewTextView().SetText(infoText)

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
	case typeTable:
		fields, err := v.ctrl.getTableColumns(ref.DB, ref.Table)
		if err != nil {
			v.showError("Listing columns for %s failed: %v", ref.Table, err)
		}

		for _, field := range fields {
			node.AddChild(tview.NewTreeNode(field.Name + " (" + field.Type + ")"))
		}
	}
}

func (v *mainView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyCtrlQ:
		v.app.Stop()
	case event.Key() == tcell.KeyCtrlI:
		v.app.SetFocus(v.queryInput)
	case event.Key() == tcell.KeyCtrlT:
		v.app.SetFocus(v.dbTree)
	case event.Key() == tcell.KeyCtrlR:
		v.app.SetFocus(v.resultTable)
	case event.Key() == tcell.KeyCtrlS:
		currentNode := v.dbTree.GetCurrentNode()
		if currentNode != nil {
			ref, ok := currentNode.GetReference().(*nodeRef)
			if ok {
				if ref.Type == typeDB {
					v.currentDB = ref.DB
				}
			}
		}
	case event.Key() == tcell.KeyCtrlA:
		v.addDatabaseDialog()
	case event.Key() == tcell.KeyCtrlSpace:
		if v.currentDB == "" {
			v.showError("No database has been selected")

			return nil
		}

		if err := v.ctrl.execQuery(v.currentDB, v.queryInput.GetText()); err != nil {
			v.showError("Query failed: %v", err)

			return nil
		}

		v.app.SetFocus(v.resultTable)
	default:
		return event
	}

	return nil
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
		v.currentDB = dbID
	}
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
