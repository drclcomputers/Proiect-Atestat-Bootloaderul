package main

import "time"

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Salt         string
	IsAdmin      bool
	CreatedAt    time.Time
}

type Article struct {
	ID          int64
	Slug        string
	Title       string
	Summary     string
	Content     string
	PublishedAt time.Time
}

type Comment struct {
	ID        int64
	ArticleID int64
	UserID    int64
	Username  string
	Content   string
	CreatedAt time.Time
}

type QuizQuestion struct {
	ID            int64
	Question      string
	OptionA       string
	OptionB       string
	OptionC       string
	OptionD       string
	CorrectOption string
	Explanation   string
}

type QuizResult struct {
	ID       int64
	UserID   int64
	Score    int
	Total    int
	TakenAt  time.Time
}

type PageData struct {
	Title      string
	User       *User
	Flash      string
	FlashKind  string // "success" sau "error"
	Data       interface{}
}
