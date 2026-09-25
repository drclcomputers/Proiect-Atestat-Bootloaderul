package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

var (
	db  *sql.DB
	tpl *template.Template
)

func main() {
	var err error
	db, err = sql.Open("sqlite", "atestat.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	initSchema(db)
	seedData(db)

	tpl, err = template.New("").Funcs(template.FuncMap{
		"markdown": renderMarkdown,
	}).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("nu am putut încărca template-urile: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public
	mux.HandleFunc("GET /{$}", homeHandler)
	mux.HandleFunc("GET /articles", articlesListHandler)
	mux.HandleFunc("GET /articles/{slug}", articleDetailHandler)
	mux.HandleFunc("POST /articles/{slug}/comments", addCommentHandler)

	mux.HandleFunc("GET /quiz", quizPageHandler)
	mux.HandleFunc("POST /quiz/submit", quizSubmitHandler)

	mux.HandleFunc("GET /register", registerPageHandler)
	mux.HandleFunc("POST /register", registerHandler)
	mux.HandleFunc("GET /login", loginPageHandler)
	mux.HandleFunc("POST /login", loginHandler)
	mux.HandleFunc("POST /logout", logoutHandler)

	// Admin
	mux.HandleFunc("GET /admin", requireAdmin(adminDashboardHandler))

	mux.HandleFunc("GET /admin/articles", requireAdmin(adminArticlesHandler))
	mux.HandleFunc("GET /admin/articles/new", requireAdmin(adminArticleNewPageHandler))
	mux.HandleFunc("POST /admin/articles/new", requireAdmin(adminArticleCreateHandler))
	mux.HandleFunc("GET /admin/articles/{id}/edit", requireAdmin(adminArticleEditPageHandler))
	mux.HandleFunc("POST /admin/articles/{id}/edit", requireAdmin(adminArticleUpdateHandler))
	mux.HandleFunc("POST /admin/articles/{id}/delete", requireAdmin(adminArticleDeleteHandler))

	mux.HandleFunc("GET /admin/quiz", requireAdmin(adminQuizHandler))
	mux.HandleFunc("POST /admin/quiz/new", requireAdmin(adminQuizCreateHandler))
	mux.HandleFunc("POST /admin/quiz/{id}/delete", requireAdmin(adminQuizDeleteHandler))

	mux.HandleFunc("GET /admin/comments", requireAdmin(adminCommentsHandler))
	mux.HandleFunc("POST /admin/comments/{id}/delete", requireAdmin(adminCommentDeleteHandler))

	addr := ":8080"
	log.Println("Server pornit → http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%v)", r.Method, r.URL.Path, time.Since(start))
	})
}

func render(w http.ResponseWriter, r *http.Request, name string, data PageData) {
	data.User = currentUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("eroare template %q: %v", name, err)
		http.Error(w, "eroare server", http.StatusInternalServerError)
	}
}
