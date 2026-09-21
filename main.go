package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

// Inventory item
type Item struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Data passed to the HTML page
type PageData struct {
	Items    []Item
	Total    int
	Hostname string
	Env      string
	Error    string
}

var (
	db       *sql.DB
	hostname string
	page     = template.Must(template.New("page").Parse(pageHTML))
)

// getenv reads an environment variable, falling back to a default.
// The defaults match the Vagrant VM, so nothing needs to change today,
// but in Azure we can point the app at a new database without touching code.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getenv("DB_HOST", "localhost"),
		getenv("DB_PORT", "5432"),
		getenv("DB_USER", "legacy_user"),
		getenv("DB_PASSWORD", "supersecret"),
		getenv("DB_NAME", "inventory_db"),
		getenv("DB_SSLMODE", "disable"),
	)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// sql.Open doesn't actually connect; Ping does. Fail loudly if the DB is unreachable.
	if err := db.Ping(); err != nil {
		log.Fatalf("Cannot reach database: %v", err)
	}

	hostname, _ = os.Hostname()
	setupDatabase()

	http.HandleFunc("/", homeHandler)               // HTML page
	http.HandleFunc("/add", addHandler)             // form submissions
	http.HandleFunc("/inventory", inventoryHandler) // JSON API (unchanged)
	http.HandleFunc("/healthz", healthHandler)      // health check (for Kubernetes later)

	addr := ":" + getenv("PORT", "8080")
	log.Printf("TaskMinds Inventory running on %s (host: %s)", addr, hostname)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func setupDatabase() {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS inventory (
		id    SERIAL PRIMARY KEY,
		name  TEXT NOT NULL,
		count INT  NOT NULL
	);`)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	}

	// Only seed sample data when the table is empty.
	// (The old ON CONFLICT DO NOTHING never triggered, because no column is UNIQUE,
	// so every restart added another "Server Racks" row.)
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM inventory").Scan(&n); err != nil {
		log.Fatalf("Error counting rows: %v", err)
	}
	if n == 0 {
		_, err := db.Exec(`INSERT INTO inventory (name, count) VALUES
			('Server Racks', 42), ('Network Switches', 18), ('Patch Cables', 250)`)
		if err != nil {
			log.Fatalf("Error seeding data: %v", err)
		}
		log.Println("Seeded sample inventory")
	}
}

func fetchItems() ([]Item, error) {
	rows, err := db.Query("SELECT id, name, count FROM inventory ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.Count); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := PageData{
		Hostname: hostname,
		Env:      getenv("APP_ENV", "On-Prem · Vagrant VM"),
	}
	items, err := fetchItems()
	if err != nil {
		data.Error = err.Error()
	}
	data.Items = items
	for _, it := range items {
		data.Total += it.Count
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Execute(w, data); err != nil {
		log.Printf("template error: %v", err)
	}
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	count, err := strconv.Atoi(r.FormValue("count"))
	if name == "" || err != nil || count < 0 {
		http.Error(w, "Please enter an item name and a count of 0 or more.", http.StatusBadRequest)
		return
	}
	if _, err := db.Exec("INSERT INTO inventory (name, count) VALUES ($1, $2)", name, count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Redirect back to the page so a browser refresh doesn't resubmit the form
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func inventoryHandler(w http.ResponseWriter, r *http.Request) {
	items, err := fetchItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

const pageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>TaskMinds Inventory</title>
<style>
  :root {
    --bg: #f4f6f9; --card: #ffffff; --text: #1d2433; --muted: #6b7385;
    --border: #e2e6ee; --accent: #2f6fed; --accent-soft: #e8f0ff; --danger: #c0392b;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #11151c; --card: #1a202b; --text: #e6e9ef; --muted: #9aa3b5;
      --border: #2a3242; --accent: #5b8def; --accent-soft: #1f2b44; --danger: #e57366;
    }
  }
  * { box-sizing: border-box; }
  body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
         background: var(--bg); color: var(--text); }
  .wrap { max-width: 820px; margin: 0 auto; padding: 32px 20px; }
  header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 24px; }
  h1 { margin: 0; font-size: 1.6rem; }
  .sub { color: var(--muted); margin-top: 4px; font-size: 0.95rem; }
  .badge { background: var(--accent-soft); color: var(--accent); padding: 6px 12px; border-radius: 999px;
           font-size: 0.85rem; font-weight: 600; white-space: nowrap; }
  .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin-bottom: 20px; }
  .stat, .card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; }
  .stat { padding: 16px; }
  .stat .label { color: var(--muted); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; }
  .stat .value { font-size: 1.6rem; font-weight: 700; margin-top: 4px; }
  .card { padding: 20px; margin-bottom: 20px; overflow-x: auto; }
  h2 { margin: 0 0 14px; font-size: 1.1rem; }
  table { width: 100%; border-collapse: collapse; }
  th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid var(--border); }
  th { color: var(--muted); font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.04em; }
  td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
  tr:last-child td { border-bottom: none; }
  form { display: flex; flex-wrap: wrap; gap: 10px; }
  input { flex: 1 1 180px; padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px;
          background: var(--bg); color: var(--text); font-size: 1rem; }
  input[type=number] { flex: 0 1 120px; }
  button { padding: 10px 18px; border: none; border-radius: 8px; background: var(--accent); color: #fff;
           font-size: 1rem; font-weight: 600; cursor: pointer; }
  button:hover { filter: brightness(1.1); }
  .error { color: var(--danger); }
  .empty { color: var(--muted); text-align: center; padding: 20px; }
  footer { color: var(--muted); font-size: 0.85rem; text-align: center; }
  footer code { background: var(--card); border: 1px solid var(--border); padding: 2px 6px; border-radius: 4px; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <div>
      <h1>📦 TaskMinds Inventory</h1>
      <div class="sub">Internal inventory system</div>
    </div>
    <span class="badge">{{.Env}}</span>
  </header>

  <div class="stats">
    <div class="stat"><div class="label">Product lines</div><div class="value">{{len .Items}}</div></div>
    <div class="stat"><div class="label">Total units</div><div class="value">{{.Total}}</div></div>
    <div class="stat"><div class="label">Served by</div><div class="value" style="font-size:1rem;word-break:break-all">{{.Hostname}}</div></div>
  </div>

  {{if .Error}}<div class="card error">Database error: {{.Error}}</div>{{end}}

  <div class="card">
    <h2>Stock</h2>
    <table>
      <thead><tr><th>ID</th><th>Item</th><th class="num">Count</th></tr></thead>
      <tbody>
      {{range .Items}}
        <tr><td>{{.ID}}</td><td>{{.Name}}</td><td class="num">{{.Count}}</td></tr>
      {{else}}
        <tr><td colspan="3" class="empty">No items yet — add one below.</td></tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>Add item</h2>
    <form method="POST" action="/add">
      <input name="name" placeholder="Item name" required maxlength="100">
      <input name="count" type="number" min="0" placeholder="Count" required>
      <button type="submit">Add</button>
    </form>
  </div>

  <footer>JSON API: <code>/inventory</code> · Health: <code>/healthz</code></footer>
</div>
</body>
</html>`