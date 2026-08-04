package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, _ := sql.Open("sqlite3", "/app/working/workspaces/default/ModuForge/backend/data/moduforge.db?_journal_mode=WAL")
	defer db.Close()

	rows, _ := db.Query("SELECT id, user_id, widget_type, position_y FROM dashboard_widgets ORDER BY user_id, position_y")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var uid, wt string
			var py int
			rows.Scan(&id, &uid, &wt, &py)
			fmt.Printf("[%d] user=%s type=%s pos=%d\n", id, uid[:8], wt, py)
		}
	}
}
