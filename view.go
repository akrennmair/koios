package main

import (
	"fmt"

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

	dbRootNode *tview.TreeNode

	currentResultTableRow int
}

type nodeRef struct {
	Type nodeType
	Name string
}

type nodeType int

const (
	typeDB nodeType = iota
	typeTable
)

func newMainView() *mainView {
	return &mainView{}
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

	v.layout = tview.NewFlex().
		AddItem(v.dbTree, 0, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(v.queryInput, 0, 1, false).
			AddItem(v.resultTable, 0, 3, false), 0, 3, false)

	v.app.SetInputCapture(v.handleKey)

	v.app.SetRoot(v.layout, true).EnableMouse(true)
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
		tables, err := v.ctrl.getTables()
		if err != nil {
			v.showError("Listing tables failed: %v", err)
			return
		}
		for _, table := range tables {
			node.AddChild(tview.NewTreeNode(table).SetSelectable(true).SetReference(&nodeRef{Type: typeTable, Name: table}))
		}
	case typeTable:
		fields, err := v.ctrl.getTableColumns(ref.Name)
		if err != nil {
			v.showError("Listing columns for %s failed: %v", ref.Name, err)
		}

		for _, field := range fields {
			node.AddChild(tview.NewTreeNode(field.Name + " (" + field.Type + ")"))
		}
	}
}

func (v *mainView) handleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlI:
		v.app.SetFocus(v.queryInput)
	case tcell.KeyCtrlK:
		v.app.SetFocus(v.dbTree)
	case tcell.KeyCtrlSpace:
		if err := v.ctrl.execQuery(v.queryInput.GetText()); err != nil {
			v.showError("Query failed: %v", err)
			return nil
		}
	default:
		return event
	}

	return nil
}

func (v *mainView) addDatabase(db string) {
	v.dbRootNode.AddChild(tview.NewTreeNode(db).SetSelectable(true).SetReference(&nodeRef{Type: typeDB, Name: db}))
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
}

func (v *mainView) showError(s string, args ...any) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf(s, args...)).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			v.app.SetRoot(v.layout, true)
		})

	v.app.SetRoot(modal, false)
}

func (v *mainView) run() error {
	if err := v.app.Run(); err != nil {
		return fmt.Errorf("running application failed: %w", err)
	}
	return nil
}
