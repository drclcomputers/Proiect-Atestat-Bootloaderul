# Bootloader — site de atestat

Site interactiv (Go + SQLite) care documentează proiectul de bootloader:
jurnal de dezvoltare, quiz interactiv, conturi de utilizator, comentarii și
panou de administrare.

## Cerinte

- Go 1.22 sau mai nou (verifica cu `go version`)
- Acces la internet **doar prima data**, ca sa descarce driverul SQLite

## Instalare si rulare

```bash
cd atestat-bootloader
go mod tidy      # descarca modernc.org/sqlite (driver SQLite pur Go, fara CGO)
go run .
```

Apoi deschide **http://localhost:8080** in browser.

La prima pornire se creeaza automat fisierul `atestat.db` (SQLite), cu:
- un cont de admin: **utilizator `admin`, parola `admin123`** — schimba parola
  dupa prima logare (poti adauga un formular de schimbare a parolei, sau
  editeaza direct in DB pentru inceput)
- 3 articole demo despre bootloader (poti sa le editezi/stergi din panoul de admin)
- 5 intrebari de quiz demo

## Structura proiectului

```
main.go          - rutare HTTP si pornirea serverului
db.go            - conectare SQLite, creare schema, date demo
models.go        - structurile de date (User, Article, Comment, QuizQuestion...)
auth.go          - inregistrare, autentificare, sesiuni, parole
articles.go      - jurnal de dezvoltare + comentarii
quiz.go          - quiz interactiv
admin.go         - panou de administrare (CRUD articole/quiz/comentarii)
schema.sql       - schema bazei de date
templates/       - pagini HTML (html/template)
static/style.css - stil (tema terminal, intunecata)
```

## Functionalitati

- **Vizitatori**: citesc articolele si dau quiz-ul (scorul nu se salveaza
  daca nu sunt autentificati)
- **Cont utilizator**: comenteaza la articole, scorurile la quiz se salveaza
  in istoric (`quiz_results`)
- **Admin**: adauga/editeaza/sterge articole, intrebari de quiz si modereaza
  comentariile, din `/admin`

## Pentru lucrarea scrisa

Cateva idei de subiecte pe care le poti detalia in documentatie:
- de ce SQLite in loc de MySQL/Postgres (fisier unic, zero configurare, potrivit
  pentru un proiect mic si portabil)
- cum functioneaza sesiunile (cookie cu token aleator, stocat in tabela `sessions`)
- schema relationala (diagrame ER pentru users/articles/comments/quiz)
- de ce parolele sunt hash-uite cu salt (niciodata stocate in clar) — poti
  mentiona ca intr-un sistem de productie s-ar folosi bcrypt/argon2 in loc de
  SHA-256+salt, pentru rezistenta la atacuri prin forta bruta

## Securitate — de stiut

Codul e facut pentru claritate si scop didactic. Daca vrei sa mergi mai
departe (sau daca profesorul intreaba), puncte de imbunatatit:
- hashing parole cu `bcrypt` (`golang.org/x/crypto/bcrypt`) in loc de SHA-256
- protectie CSRF pe formulare
- validare/sanitizare mai stricta a input-ului
