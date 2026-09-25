package main

import (
	"net/http"
	"strconv"
)

func listQuizQuestions() ([]QuizQuestion, error) {
	rows, err := db.Query(`SELECT id, question, option_a, option_b, option_c, option_d, correct_option, explanation FROM quiz_questions ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var qs []QuizQuestion
	for rows.Next() {
		var q QuizQuestion
		if err := rows.Scan(&q.ID, &q.Question, &q.OptionA, &q.OptionB, &q.OptionC, &q.OptionD, &q.CorrectOption, &q.Explanation); err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	return qs, nil
}

func quizPageHandler(w http.ResponseWriter, r *http.Request) {
	questions, err := listQuizQuestions()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	render(w, r, "quiz", PageData{Title: "Quiz", Data: questions})
}

type quizResultData struct {
	Questions []QuizQuestion
	Answers   map[int64]string
	Score     int
	Total     int
}

func quizSubmitHandler(w http.ResponseWriter, r *http.Request) {
	questions, err := listQuizQuestions()
	if err != nil {
		http.Error(w, "eroare server", http.StatusInternalServerError)
		return
	}
	r.ParseForm()

	answers := make(map[int64]string)
	score := 0
	for _, q := range questions {
		chosen := r.FormValue("q" + strconv.FormatInt(q.ID, 10))
		answers[q.ID] = chosen
		if chosen == q.CorrectOption {
			score++
		}
	}

	if u := currentUser(r); u != nil {
		db.Exec(`INSERT INTO quiz_results (user_id, score, total) VALUES (?, ?, ?)`, u.ID, score, len(questions))
	}

	render(w, r, "quiz-result", PageData{
		Title: "Rezultat quiz",
		Data: quizResultData{
			Questions: questions,
			Answers:   answers,
			Score:     score,
			Total:     len(questions),
		},
	})
}
