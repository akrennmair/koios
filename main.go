package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	_ "modernc.org/sqlite"
)

type nodeRef struct {
	Type nodeType
	Name string
}

type nodeType int

const (
	typeDB nodeType = iota
	typeTable
)

func main() {
	ctx := context.Background()

	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatalf("No input database provided")
	}

	db, err := sqlx.Open("sqlite", flag.Arg(0))
	if err != nil {
		log.Fatalf("Couldn't open %s: %v", flag.Arg(0), err)
	}

	dbRootNode := tview.NewTreeNode("Databases")
	dbNode := tview.NewTreeNode(flag.Arg(0)).SetSelectable(true).SetReference(&nodeRef{Type: typeDB, Name: flag.Arg(0)})

	dbRootNode.AddChild(dbNode)

	dbRootNode.CollapseAll()

	app := tview.NewApplication()

	dbTree := tview.NewTreeView()
	dbTree.SetBorder(true).SetTitle("Databases")

	dbTree.SetSelectedFunc(func(node *tview.TreeNode) {
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
			rows, err := db.Query("PRAGMA table_list")
			if err != nil {
				log.Fatalf("Querying list of tables failed: %v", err)
			}

			for rows.Next() {
				var (
					schema, name, typ string
					ncol              int
					wr                int
					strict            bool
				)
				if err := rows.Scan(&schema, &name, &typ, &ncol, &wr, &strict); err != nil {
					log.Fatalf("Scan failed: %v", err)
				}
				node.AddChild(tview.NewTreeNode(name).SetSelectable(true).SetReference(&nodeRef{Type: typeTable, Name: name}))
			}

			rows.Close()
		case typeTable:
			rows, err := db.Query("PRAGMA table_info(" + ref.Name + ")")
			if err != nil {
				log.Fatalf("Querying list of columns for %s failed: %v", ref.Name, err)
			}

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
				node.AddChild(tview.NewTreeNode(name + " (" + typ + ")"))
			}
		}
	})

	queryInput := tview.NewTextArea()
	queryInput.SetBorder(true).SetTitle("Query")

	resultTable := tview.NewTable()
	resultTable.SetBorder(true).SetTitle("Result")
	resultTable.SetBorders(true)

	dbTree.SetRoot(dbRootNode).SetCurrentNode(dbRootNode)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlI:
			app.SetFocus(queryInput)
			return nil
		case tcell.KeyCtrlK:
			app.SetFocus(dbTree)
			return nil
		case tcell.KeyCtrlSpace:
			rows, err := db.QueryxContext(ctx, queryInput.GetText())
			if err != nil {
				// TODO: show error.
				return nil
			}
			resultTable.Clear()

			columns, err := rows.Columns()
			if err != nil {
				// TODO: show error
				return nil
			}

			for i := 0; i < len(columns); i++ {
				resultTable.SetCell(0, i, tview.NewTableCell(columns[i]).SetAttributes(tcell.AttrBold))
			}
			rowIdx := 1
			defer rows.Close()
			for rows.Next() {
				row, err := rows.SliceScan()
				if err != nil {
					// TODO: show error.
					return nil
				}
				for idx, v := range row {
					resultTable.SetCell(rowIdx, idx, tview.NewTableCell(fmt.Sprint(v)))
				}
				rowIdx++
			}
			return nil
		}
		return event
	})

	flex := tview.NewFlex().
		AddItem(dbTree, 0, 1, true).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(queryInput, 0, 1, false).
			AddItem(resultTable, 0, 3, false), 0, 3, false)
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
