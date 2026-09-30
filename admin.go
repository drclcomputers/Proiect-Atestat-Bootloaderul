package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

type adminStats struct {
	UserCount    int
	ArticleCount int
	CommentCount int
	QuizCount    int
	ResultCount  int
}

type quizResultRow struct {
	ID       int64
	UserID   int64
	Username string
	Score    int
	Total    int
	Percent  int
	TakenAt  time.Time
}

type quizResultsView struct {
	Results  []quizResultRow
	Attempts int
	AvgPct   int
	BestPct  int
}

func adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	var s adminStats
	db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&s.UserCount)
	db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&s.ArticleCount)
	db.QueryRow(`SELECT COUNT(*) FROM comments`).Scan(&s.CommentCount)
	db.QueryRow(`SELECT COUNT(*) FROM quiz_questions`).Scan(&s.QuizCount)
	db.QueryRow(`SELECT COUNT(*) FROM quiz_results`).Scan(&s.ResultCount)
	render(w, r, "admin-dashboard", PageData{Title: "Panou admin", Data: s})
}

// Articole
func adminArticlesHandler(w http.ResponseWriter, r *http.Request) {
	articles, err := listArticles()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "admin-articles", PageData{Title: "Administrare articole", Data: articles})
}

func adminArticleNewPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "admin-article-form", PageData{Title: "Articol nou", Data: Article{}})
}

func adminArticleCreateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	title := r.FormValue("title")
	summary := r.FormValue("summary")
	content := r.FormValue("content")
	slug := slugify(title)

	_, err := db.Exec(
		`INSERT INTO articles (slug, title, summary, content) VALUES (?, ?, ?, ?)`,
		slug, title, summary, content,
	)
	if err != nil {
		render(w, r, "admin-article-form", PageData{Title: "Articol nou", Flash: "Nu am putut salva (poate exista deja un articol cu acest titlu).", FlashKind: "error", Data: Article{Title: title, Summary: summary, Content: content}})
		return
	}
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func adminArticleEditPageHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	a := &Article{}
	err := db.QueryRow(`SELECT id, slug, title, summary, content, published_at FROM articles WHERE id = ?`, id).
		Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.Content, &a.PublishedAt)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "admin-article-form", PageData{Title: "Editeaza articol", Data: *a})
}

func adminArticleUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	r.ParseForm()
	title := r.FormValue("title")
	summary := r.FormValue("summary")
	content := r.FormValue("content")

	_, err := db.Exec(
		`UPDATE articles SET title = ?, summary = ?, content = ? WHERE id = ?`,
		title, summary, content, id,
	)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

func adminArticleDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	db.Exec(`DELETE FROM articles WHERE id = ?`, id)
	http.Redirect(w, r, "/admin/articles", http.StatusSeeOther)
}

// Quiz
func adminQuizHandler(w http.ResponseWriter, r *http.Request) {
	questions, err := listQuizQuestions()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "admin-quiz", PageData{Title: "Administrare quiz", Data: questions})
}

func adminQuizCreateHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	_, err := db.Exec(
		`INSERT INTO quiz_questions (question, option_a, option_b, option_c, option_d, correct_option, explanation)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.FormValue("question"), r.FormValue("option_a"), r.FormValue("option_b"),
		r.FormValue("option_c"), r.FormValue("option_d"), strings.ToUpper(r.FormValue("correct_option")),
		r.FormValue("explanation"),
	)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/quiz", http.StatusSeeOther)
}

func adminQuizDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	db.Exec(`DELETE FROM quiz_questions WHERE id = ?`, id)
	http.Redirect(w, r, "/admin/quiz", http.StatusSeeOther)
}

func adminQuizResultsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		`SELECT r.id, r.user_id, u.username, r.score, r.total, r.taken_at
		 FROM quiz_results r
		 JOIN users u ON u.id = r.user_id
		 ORDER BY r.taken_at DESC`,
	)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var v quizResultsView
	sum := 0
	for rows.Next() {
		var x quizResultRow
		if err := rows.Scan(&x.ID, &x.UserID, &x.Username, &x.Score, &x.Total, &x.TakenAt); err != nil {
			http.Error(w, "eroare server", http.StatusInternalServerError)
			return
		}
		if x.Total > 0 {
			x.Percent = x.Score * 100 / x.Total
		}
		sum += x.Percent
		if x.Percent > v.BestPct {
			v.BestPct = x.Percent
		}
		v.Results = append(v.Results, x)
	}
	v.Attempts = len(v.Results)
	if v.Attempts > 0 {
		v.AvgPct = sum / v.Attempts
	}

	render(w, r, "admin-quiz-results", PageData{Title: "Rezultate quiz", Data: v})
}

// Comentarii
func adminCommentsHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		`SELECT c.id, c.article_id, a.title, c.user_id, u.username, c.content, c.created_at
		 FROM comments c
		 JOIN users u ON u.id = c.user_id
		 JOIN articles a ON a.id = c.article_id
		 ORDER BY c.created_at DESC`,
	)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		Comment
		ArticleTitle string
	}
	var comments []row
	for rows.Next() {
		var c row
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.ArticleTitle, &c.UserID, &c.Username, &c.Content, &c.CreatedAt); err != nil {
			http.Error(w, "eroare server", http.StatusInternalServerError)
			return
		}
		comments = append(comments, c)
	}
	render(w, r, "admin-comments", PageData{Title: "Administrare comentarii", Data: comments})
}

func adminCommentDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	db.Exec(`DELETE FROM comments WHERE id = ?`, id)
	http.Redirect(w, r, "/admin/comments", http.StatusSeeOther)
}

// schimbare parolă useri (doar admin)
func adminUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := listUsers()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "admin-users", PageData{Title: "Administrare utilizatori", Data: users})
}

func adminChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	r.ParseForm()
	newPassword := r.FormValue("password")

	if err := updateUserPassword(id, newPassword); err != nil {
		users, _ := listUsers()
		render(w, r, "admin-users", PageData{
			Title:     "Administrare utilizatori",
			Flash:     err.Error(),
			FlashKind: "error",
			Data:      users,
		})
		return
	}
	http.Redirect(w, r, "/admin/users?ok=1", http.StatusSeeOther)
}

func adminUserDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	current := currentUser(r)

	if current != nil && current.ID == id {
		users, _ := listUsers()
		render(w, r, "admin-users", PageData{
			Title:     "Administrare utilizatori",
			Flash:     "Nu poți șterge contul cu care ești autentificat.",
			FlashKind: "error",
			Data:      users,
		})
		return
	}

	db.Exec(`DELETE FROM users WHERE id = ?`, id)
	http.Redirect(w, r, "/admin/users?ok=deleted", http.StatusSeeOther)
}

func adminToggleAdminHandler(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	current := currentUser(r)

	refuse := func(msg string) {
		users, _ := listUsers()
		render(w, r, "admin-users", PageData{
			Title: "Administrare utilizatori", Flash: msg, FlashKind: "error", Data: users,
		})
	}

	if id == mainAdminID {
		refuse("Contul de admin principal nu poate fi modificat.")
		return
	}
	if current != nil && current.ID == id {
		refuse("Nu îți poți schimba propriul rol.")
		return
	}

	if _, err := db.Exec(`UPDATE users SET is_admin = 1 - is_admin WHERE id = ?`, id); err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/users?ok=role", http.StatusSeeOther)
}
