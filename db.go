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

// initSchema creeaza baza de date daca nu exista
func initSchema(db *sql.DB) {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("nu am putut crea schema: %v", err)
	}
}

// seedData creeaza contul de admin si quiz-ul doar daca nu exista, gen cand este creata baza de date
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
	log.Println("Cont admin creat -> utilizator: admin | parola: admin123 (schimb-o din panoul de admin sau direct in DB)")

	// Articole
	articles := []struct{ slug, title, summary, content string }{
		{
			"ce-este-un-bootloader",
			"Ce este un bootloader?",
			"O introducere în procesul de pornire al unui calculator: BIOS, MBR și primii pași spre un sistem de operare.",
			"Când pornești calculatorul, procesorul execută codul aflat la o adresă fixă în memorie, controlat inițial de firmware-ul BIOS (Basic Input Output System). " +
				"BIOS-ul caută un dispozitiv de boot valid (Hard Drive, CD, FDD, Stick USB, etc) și încarcă primii 512 de bytes ai acestuia (Master Boot Record) la adresa 0x7C00, " +
				"apoi predă controlul acolo dacă ultimii doi bytes conțin semnatura 0xAA55.\n\n" +
				"Bootloaderul este exact acest cod de 512 bytes: prima bucata de software scrisă de programator care ruleaza pe masină. " +
				"Rolul lui este să pregătească terenul pentru sistemul de operare: afișează mesaje, inițializează hardware minim si, " +
				"cel mai important, trece procesorul din real mode (16-bit, moștenit din anii '80 de pe Intel 8086) în protected mode (32-bit), " +
				"unde poate accesa toata memoria disponibila (de precizat că limita maximă pentru 32 de biți este sub 4GB, undeva la 3.5GB) și poate rula cod modern.",
		},
		{
			"real-mode-vs-protected-mode",
			"Real mode vs Protected mode",
			"Diferențele esențiale între cele două moduri de funcționare ale procesorului x86 și de ce bootloaderul trebuie să treacă prin ambele.",
			"La pornire, un procesor x86 începe întotdeauna în real mode, din motive de compatibilitate cu sistemele vechi (anii 80'). " +
				"În real mode adresele de memorie sunt calculate din perechi \"segment:offset\" și sunt limitate la 1 MB de memorie adresabilă (teoretic doar 1MB, dar adesea doar 640KB, restul necesitând niște 'artificii').\n\n" +
				"Protected mode elimină această limitare: folosește adrese pe 32 de biți, oferă protecție a memoriei între procese (dacă un program se blochează, nu blochează tot sistemul) și acces la " +
				"toată memoria fizică (teoretic 4GB, practic doar 3.5GB). Trecerea între cele două moduri se face prin setarea bitului 'PE' din registrul 'CR0', dar înainte de asta " +
				"trebuie construit un GDT (Global Descriptor Table) care descrie segmentele de cod și date pe care le va folosi procesorul.",
		},
		{
			"jurnal-progres",
			"Jurnal de progres",
			"Etapele parcurse pana acum in dezvoltarea bootloaderului si urmatorii pasi planificati.",
			"Etapa 1: bootloader minimal care afiseaza un mesaj pe ecran folosind intreruperea BIOS int 0x10, testat in QEMU.\n\n" +
				"Etapa 2: construirea unui GDT valid si trecerea in protected mode.\n\n" +
				"Etapa 3 (planificata): incarcarea unui al doilea stagiu de pe disc, deoarece 512 bytes sunt insuficienti pentru mai mult " +
				"decat pasii de baza.\n\n" +
				"Pas urmator, dincolo de atestat: acest bootloader este primul caramid pentru ASMOS, un proiect personal de sistem de operare " +
				"la care lucrez in continuare.",
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

	// Intrebari quiz
	questions := []struct {
		q, a, b, c, d, correct, explanation string
	}{
		{
			"Cati bytes are Master Boot Record-ul (MBR)?",
			"256 bytes", "512 bytes", "1024 bytes", "4096 bytes",
			"B",
			"MBR-ul ocupa exact un sector de disc: 512 bytes, dintre care ultimii 2 sunt semnatura 0x55AA.",
		},
		{
			"Ce intrerupere BIOS este folosita de obicei pentru afisarea de text in real mode?",
			"int 0x13", "int 0x21", "int 0x10", "int 0x80",
			"C",
			"int 0x10 este intreruperea de servicii video a BIOS-ului, folosita printre altele pentru afisarea caracterelor pe ecran.",
		},
		{
			"Care structura trebuie definita inainte de a trece in protected mode?",
			"Un stack pointer", "Un GDT (Global Descriptor Table)", "Un fisier de configurare", "O tabela de rutare",
			"B",
			"GDT-ul descrie segmentele de memorie (cod, date) pe care procesorul le va folosi in protected mode.",
		},
		{
			"La ce adresa de memorie incarca BIOS-ul, in mod traditional, bootloaderul?",
			"0x0000", "0x7C00", "0xFFFF", "0x1000",
			"B",
			"Conventia mostenita de la primele PC-uri IBM este incarcarea la adresa 0x7C00 in memorie.",
		},
		{
			"Care este principala limitare a real mode-ului pe care protected mode-ul o rezolva?",
			"Viteza procesorului", "Adresarea limitata la 1 MB de memorie", "Lipsa suportului pentru tastatura", "Lipsa unei surse de alimentare",
			"B",
			"In real mode, adresarea segment:offset limiteaza memoria accesibila la aproximativ 1 MB.",
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
