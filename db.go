package main

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"log"
)

//go:embed schema.sql
var schemaSQL string

// initSchema creează baza de date dacă nu există
func initSchema(db *sql.DB) {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("nu am putut crea schema: %v", err)
	}
}

// seedData creează contul de admin și quiz-ul doar dacă nu există, gen când este creată baza de date
func seedData(db *sql.DB) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count > 0 {
		return // deja populat, nu suprascrie
	}

	// Cont admin implicit
	salt := randomHex(16)
	hash := hashPassword("admin123", salt)
	_, err := db.Exec(
		`INSERT INTO users (username, password_hash, salt, is_admin) VALUES (?, ?, ?, 1)`,
		"admin", hash, salt,
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Cont admin default creat -> utilizator: admin | parolă: admin123")

	// Articole
	articles := []struct{ slug, title, summary, content string }{
		{
			"ce-este-un-bootloader",
			"Ce este un bootloader?",
			"O introducere în procesul de pornire al unui calculator: BIOS, MBR și primii pași spre un sistem de operare.",
			"Când pornești calculatorul, procesorul execută codul aflat la o adresă fixă în memorie, controlat inițial de firmware-ul BIOS (Basic Input Output System). " +
				"BIOS-ul caută un dispozitiv de boot valid (Hard Drive, CD, FDD, Stick USB, etc) și încarcă primii 512 de octeți ai acestuia (Master Boot Record) la adresa 0x7C00, " +
				"apoi predă controlul acolo dacă ultimii doi octeți conțin semnătura 0xAA55.\n\n" +
				"Bootloaderul este exact acest cod de 512 octeți: prima bucată de software scrisă de programator care rulează pe mașină. " +
				"Rolul lui este să pregătească terenul pentru sistemul de operare: afișează mesaje, inițializează hardware minim și, " +
				"cel mai important, trece procesorul din real mode (16-bit, moștenit din anii '80 de pe Intel 8086) în protected mode (32-bit), " +
				"unde poate accesa toată memoria disponibilă (de precizat că limita maximă pentru 32 de biți este sub 4GB, undeva la 3.5GB) și poate rula cod modern.",
		},
		{
			"real-mode-vs-protected-mode",
			"Real mode vs Protected mode",
			"Diferențele esențiale între cele două moduri de funcționare ale procesorului x86 și de ce bootloaderul trebuie să treacă prin ambele.",
			"La pornire, un procesor x86 începe întotdeauna în real mode, din motive de compatibilitate cu sistemele vechi (anii '80). " +
				"În real mode adresele de memorie sunt calculate din perechi \"segment:offset\" și sunt limitate la 1 MB de memorie adresabilă (teoretic doar 1MB, dar adesea doar 640KB, restul necesitând niște 'artificii').\n\n" +
				"Protected mode elimină această limitare: folosește adrese pe 32 de biți, oferă protecție a memoriei între procese (dacă un program se blochează, nu blochează tot sistemul) și acces la " +
				"toată memoria fizică (teoretic 4GB, practic doar 3.5GB). Trecerea între cele două moduri se face prin setarea bitului 'PE' din registrul 'CR0', dar înainte de asta " +
				"trebuie construit un GDT (Global Descriptor Table) care descrie segmentele de cod și date pe care le va folosi procesorul.",
		},
		{
			"jurnal-progres",
			"Jurnal de progres",
			"Etapele parcurse până acum în dezvoltarea bootloaderului și următorii pași planificați.",
			"Etapa 1: bootloader minimal care afișează un mesaj pe ecran folosind întreruperea BIOS int 0x10, testat în QEMU.\n\n" +
				"Etapa 2: construirea unui GDT valid și trecerea în protected mode.\n\n" +
				"Etapa 3 (planificată): încărcarea unui al doilea stagiu de pe disc, deoarece 512 octeți sunt insuficienți pentru mai mult " +
				"decât pașii de bază.\n\n" +
				"Pasul următor: integrarea limbajului de programare C pentru a începe să-ți scrii propriile funcții și librării.",
		},
	}
	for _, a := range articles {
		_, err := db.Exec(
			`INSERT INTO articles (slug, title, summary, content) VALUES (?, ?, ?, ?)`,
			a.slug, a.title, a.summary, a.content,
		)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Întrebări quiz
	questions := []struct {
		q, a, b, c, d, correct, explanation string
	}{
		{
			"Câți octeți (bytes) are Master Boot Record-ul (MBR)?",
			"256 octeți", "512 octeți", "1024 octeți", "4096 octeți",
			"B",
			"MBR-ul ocupă exact un sector de disc: 512 octeți, dintre care ultimii 2 sunt semnătura 0x55AA.",
		},
		{
			"Ce întrerupere BIOS este folosită de obicei pentru afișarea de text în real mode?",
			"int 0x13", "int 0x21", "int 0x10", "int 0x80",
			"C",
			"int 0x10 este întreruperea de servicii video a BIOS-ului, folosită printre altele pentru afișarea caracterelor pe ecran.",
		},
		{
			"Care structură trebuie definită înainte de a trece în protected mode?",
			"Un stack pointer", "Un GDT (Global Descriptor Table)", "Un fișier de configurare", "O tabelă de rutare",
			"B",
			"GDT-ul descrie segmentele de memorie (cod, date) pe care procesorul le va folosi în protected mode.",
		},
		{
			"La ce adresă de memorie încarcă BIOS-ul, în mod tradițional, bootloaderul?",
			"0x0000", "0x7C00", "0xFFFF", "0x1000",
			"B",
			"Convenția moștenită de la primele PC-uri IBM este încărcarea la adresa 0x7C00 în memorie.",
		},
		{
			"Care este principala limitare a real mode-ului pe care protected mode-ul o rezolvă?",
			"Viteza procesorului", "Adresarea limitată la 1 MB de memorie", "Lipsa suportului pentru tastatură", "Lipsa unei surse de alimentare",
			"B",
			"În real mode, adresarea segment:offset limitează memoria accesibilă la aproximativ 1 MB.",
		},
	}
	for _, qz := range questions {
		_, err := db.Exec(
			`INSERT INTO quiz_questions (question, option_a, option_b, option_c, option_d, correct_option, explanation)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			qz.q, qz.a, qz.b, qz.c, qz.d, qz.correct, qz.explanation,
		)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(b)
}
