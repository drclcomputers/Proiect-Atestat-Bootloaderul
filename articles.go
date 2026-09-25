package main

import (
	"net/http"
)

func listArticles() ([]Article, error) {
	rows, err := db.Query(`SELECT id, slug, title, summary, content, published_at FROM articles ORDER BY published_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.Content, &a.PublishedAt); err != nil {
			return nil, err
		}
		articles = append(articles, a)
	}
	return articles, nil
}

func getArticleBySlug(slug string) (*Article, error) {
	a := &Article{}
	err := db.QueryRow(
		`SELECT id, slug, title, summary, content, published_at FROM articles WHERE slug = ?`, slug,
	).Scan(&a.ID, &a.Slug, &a.Title, &a.Summary, &a.Content, &a.PublishedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func getCommentsForArticle(articleID int64) ([]Comment, error) {
	rows, err := db.Query(
		`SELECT c.id, c.article_id, c.user_id, u.username, c.content, c.created_at
		 FROM comments c JOIN users u ON u.id = c.user_id
		 WHERE c.article_id = ? ORDER BY c.created_at ASC`, articleID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(&c.ID, &c.ArticleID, &c.UserID, &c.Username, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	articles, err := listArticles()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	if len(articles) > 3 {
		articles = articles[:3]
	}
	render(w, r, "home", PageData{Title: "Acasă", Data: articles})
}

func articlesListHandler(w http.ResponseWriter, r *http.Request) {
	articles, err := listArticles()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "articles-list", PageData{Title: "Jurnal de dezvoltare", Data: articles})
}

type articleDetailData struct {
	Article  Article
	Comments []Comment
}

func articleDetailHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	a, err := getArticleBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	comments, err := getCommentsForArticle(a.ID)
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "article-detail", PageData{
		Title: a.Title,
		Data:  articleDetailData{Article: *a, Comments: comments},
	})
}

func addCommentHandler(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	slug := r.PathValue("slug")
	if u == nil {
		http.Redirect(w, r, "/login?next=/articles/"+slug, http.StatusSeeOther)
		return
	}
	a, err := getArticleBySlug(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	content := r.FormValue("content")
	if content != "" {
		db.Exec(`INSERT INTO comments (article_id, user_id, content) VALUES (?, ?, ?)`, a.ID, u.ID, content)
	}
	http.Redirect(w, r, "/articles/"+slug, http.StatusSeeOther)
}
