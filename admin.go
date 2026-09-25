package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
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
