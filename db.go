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

// initSchema creează tabelele dacă nu există folosind fișierul schema.sql
func initSchema(db *sql.DB) {
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("nu am putut crea schema: %v", err)
	}
}

// seedData creează contul de admin și articolele dacă baza de date a fost de abia creată.
func seedData(db *sql.DB) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count > 0 {
		return // deja populat, nu suprascriem
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
	log.Println("Cont admin implicit creat -> utilizator: admin | parola: admin123")

	// Articole
	articles := []struct{ slug, title, summary, content string }{
{
			"pregatirea-spatiului-de-lucru",
			"Pregătirea spațiului de lucru",
			"Uneltele, sistemul gazdă și emulatorul de care ai nevoie înainte să scrii primul octet de bootloader.",
			`Dezvoltarea unui sistem de operare (sau măcar a primului său program, bootloaderul) începe rar cu o instrucțiune <code>jmp</code>. Începe cu o dorință puternică de învățare, FOOOARTE multă răbdare și zile sau săptămțni întregi de citit cărți, articole și urmărit tutoriale indiene la 11 noaptea. După, urmează configurarea unui mediu de lucru predictibil: un assembler, un emulator și un folder în care poți recompila în câteva secunde. Fără astea, fiecare încercare înseamnă să copiezi un fișier binar pe un stick și să repornești un calculator real (indiferent că e un 386 sau Ryzen 9 9950X) — lent spre foarte lent și, dacă greșești semnătura de boot, frustrant căci nu știi ce nu merge și de ce nu afișează nimic.

Ideea e aceeași ca în cartea din care am învățat mai tot *The little book about OS development* (Helin & Renberg): instalezi un set mic de unelte pe un sistem UNIX, apoi rulezi totul într-o **mașină virtuală**. În jurnalul de față nu folosim GRUB și un kernel ELF, ci un bootloader clasic de 512 octeți, asamblat cu NASM și pornit de QEMU ca dischetă. Uneltele se potrivesc totuși aproape 1:1, NASM + QEMU fiind mult mai simple pentru începători.

### Sistemul de operare gazdă

Toate exemplele presupun un mediu de tip UNIX:

- **Linux** (Ubuntu / Debian / Fedora) — cel mai simplu, pachetele sunt în depozitele oficiale
- **macOS** — funcționează bine cu Homebrew
- **Windows** — posibil prin WSL2 (Ubuntu în Windows); evită MinGW și alte assemblere — pot să nu funcționeze corect

Cartea originală folosește Ubuntu. Dacă vrei nicio surpriză, o mașină virtuală Ubuntu (VirtualBox sau UTM) e suficientă, chiar dacă tu lucrezi pe Windows sau pe Mac.

### Uneltele

Ai nevoie de patru lucruri:

1. **NASM** — assemblerul. Sintaxa Intel e mai lizibilă decât pe assembler-ul GNU, iar <code>-f bin</code> produce exact cei 512 octeți ai unui MBR, fără ELF, fără linker, mult mai simplu.
2. **QEMU** — emulator x86. Pornește un „PC” în câteva milisecunde, cu discheta noastră în unitatea *A:*. Alternative bune, dar mai complicat de configurat: Bochs (debugger bun; oferă o foarte precisă idee asupra instrucțiunilor executate linie cu linie de procesor) și VirtualBox (mai greoi pentru un binar de 512 octeți întrucât este folosit mai degrabă pentru mașini virtuale cu sisteme de operare mature).
3. **Un editor** — orice în care poți scrie Assembly (VS Code, Neovim, Zed — chiar și NotePad). Un plugin de syntax highlighting pentru NASM ajută la identificarea instrucțiunilor.
4. **Make** (opțional, dar util) — ca să nu tastezi de fiecare dată linia de NASM + QEMU.

Nu-ți trebuie GCC, GRUB sau <code>genisoimage</code> pentru articolele din acest jurnal. Ele apar în *littleosbook* pentru că acolo nucleul e un executabil ELF încărcat de GRUB (bootloader profesional folosit de Linux și alte sisteme de operare). Aici BIOS-ul încarcă direct sectorul 0.

### Instalare pe Ubuntu / Debian

~~~bash
sudo apt-get update
sudo apt-get install build-essential nasm qemu-system-x86 make
~~~

<code>build-essential</code> aduce Make și uneltele de compilare. <code>qemu-system-x86</code> e pachetul cu <code>qemu-system-x86_64</code>.

Verificare rapidă:

~~~bash
nasm -v
qemu-system-x86_64 --version
~~~

### Instalare pe macOS

~~~bash
brew install nasm qemu make
~~~

Comanda QEMU e tot <code>qemu-system-x86_64</code>. Dacă Homebrew nu e instalat: <code>https://brew.sh</code>.

### De ce un emulator, nu hardware real

Pe un PC fizic, ciclul e: scrii, asamblezi, copiezi pe USB, repornești, te uiți la un ecran negru, nu știi dacă a picat BIOS-ul sau bucla ta infinită. În QEMU:

- pornești din terminal, în câteva secunde
- poți opri, reface binarul, reporni
- un ecran gol **fără mesaj de eroare de boot** înseamnă că semnătura **0xAA55** a fost acceptată

Dezavantajul, recunoscut și în carte: succesul în emulator nu garantează că același binar merge pe un laptop din 2009. Am petrecut 3 zile încercând să-mi fac calculatorul să booteze de pe stick, dar într-un final am aflat că placa mea video nu suportă moduri de afișaj așa de "antice".

### Structura folderului

~~~text
bootloader/
├── start.asm      # bucla infinită (articolul "Ce este un bootloader?")
├── hello.asm      # Hello, World! prin BIOS
├── boot.asm       # trece în protected mode
└── Makefile
~~~

Makefile-ul poate arăta așa:

~~~make
.PHONY: start hello boot clean

start: start.bin
	qemu-system-x86_64 -fda start.bin

hello: hello.bin
	qemu-system-x86_64 -fda hello.bin

boot: boot.bin
	qemu-system-x86_64 -fda boot.bin

%.bin: %.asm
	nasm -f bin $< -o $@

clean:
	rm -f *.bin
~~~

<code>-fda</code> spune QEMU-ului „tratează acest fișier ca pe o dischetă”. BIOS-ul emulat citește sectorul 0, caută **0x55AA**, încarcă la **0x7C00** și sare acolo — exact lanțul descris în articolul următor.

### Primul test, înainte de orice teorie

Creează <code>start.asm</code>:

~~~asm
[org 0x7c00]
[bits 16]

start:
    jmp start

times 510-($-$$) db 0
dw 0xaa55
~~~

Apoi:

~~~bash
nasm -f bin start.asm -o start.bin
qemu-system-x86_64 -fda start.bin
~~~

Dacă QEMU deschide o fereastră neagră, fără „Boot failed”, mediul e gata. Nu s-a afișat nimic pentru că programul nu scrie pe ecran: sare la el însuși. Asta e suficient ca să știi că NASM, semnătura de boot și QEMU funcționează împreună.

### Ce nu instalăm (încă)

- **GRUB / Multiboot / ELF** — utile când treci de la un sector de 512 octeți la un nucleu C — satisfăcător, dar creează o mulțime de alte probleme și dureri de cap.
- **Bochs** — bun când vrei să inspectezi registrele după o buclă.
- **Un cross-compiler i686-elf-gcc** — necesar abia când C-ul nu mai are libc (librăria default care conține majoritatea funcțiilor uzuale din acest limbaj nu există în OS-ul nostru) și trebuie să eviți header-ele de pe gazdă.

### Ce urmează

Cu NASM și QEMU în PATH, poți deschide articolul **Ce este un bootloader?** și scrie, linie cu linie, primul program care rulează înaintea oricărui sistem de operare.`,
		},
		{
			"ce-este-un-bootloader",
			"Ce este un bootloader?",
			"Procesul de pornire al unui calculator, structura sectorului de boot și primul cod care rulează pe mașină.",
			`Atunci când apeși butonul de pornire al unui calculator, procesorul nu știe încă nimic despre sistemul de operare instalat pe disc. Primul lucru care rulează este firmware-ul plăcii de bază, numit BIOS (Basic Input Output System). Rolul BIOS-ului este să facă o verificare minimală a componentelor hardware și apoi să caute un dispozitiv de pe care poate porni sistemul: un hard disk, un SSD, un stick USB sau o dischetă Floppy.

Pentru un disc cu partiționare clasică MBR (Master Boot Record), BIOS-ul citește primul sector al discului, exact 512 octeți, îl încarcă în memorie la adresa fixă *0x7C00* și sare la această adresă, predând controlul codului aflat acolo. Acești 512 octeți formează **bootloaderul**. Dacă ultimii doi octeți din acest sector nu sunt *0x55* și *0xAA* (semnătura de boot), BIOS-ul consideră discul neinițializat și nu încearcă să pornească de pe el.

Bootloaderul este primul program care rulează pe mașină fără ajutorul niciunui sistem de operare. El trebuie să facă tot ce e nevoie folosind doar instrucțiuni de procesor și serviciile puse la dispoziție de BIOS prin întreruperi software.

La pornire, procesorul se află în **real mode**, un mod de funcționare moștenit de la procesoarele Intel 8086, în care adresele de memorie se calculează dintr-o pereche *[segment:offset]* și în care sunt disponibili doar 20 de biți de adresare, adică 1 MB de memorie. Codul din această pagină și din următoarele pornește de la acest mod.

Cel mai simplu bootloader posibil nu face nimic altceva decât să se oprească într-o buclă infinită, dar trebuie să respecte două reguli: să aibă exact 512 octeți și să se termine cu semnătura *0xAA55*.

### Exemplu: start.asm

~~~asm
[org 0x7c00]
[bits 16]

start:
    jmp start

times 510-($-$$) db 0
dw 0xaa55
~~~

Linia <code>[org 0x7c00]</code> îi spune assemblerului că acest cod va fi încărcat la adresa *0x7C00*, astfel încât toate adresele calculate în cod să fie corecte. Linia <code>[bits 16]</code> îi spune să genereze cod pentru real mode, pe 16 biți.

Eticheta <code>start</code> conține o singură instrucțiune, <code>jmp start</code>, care sare la ea însăși la nesfârșit. Linia <code>times 510-($-$$) db 0</code> umple tot spațiul rămas până la octetul 510 cu zerouri, iar <code>dw 0xaa55</code> scrie ultimii doi octeți, adică semnătura de boot.

### Cum se asamblează și rulează

~~~bash
nasm -f bin start.asm -o start.bin
qemu-system-x86_64 -fda start.bin
~~~

Dacă totul e corect, va apărea un ecran gol, fără mesaje de eroare. Bucla infinită nu afișează nimic, dar faptul că nu apare nicio eroare de boot înseamnă că BIOS-ul a găsit și a încărcat bootloaderul cu succes.`,
		},
		{
			"afisarea-de-text-cu-bios",
			"Afișarea de text cu BIOS",
			"Cum se folosește întreruperea BIOS int 0x10 pentru a scrie pe ecran, cu un bootloader complet care afișează Hello, World!",
			`Real mode oferă acces la serviciile BIOS prin întreruperi software: instrucțiunea <code>int</code>, urmată de un număr, apelează o rutină pusă la dispoziție de BIOS. Una dintre cele mai utile este **int 0x10**, care controlează afișarea video.

Dacă punem în registrul <code>AH</code> valoarea <code>0x0E</code> înainte de a apela <code>int 0x10</code>, BIOS-ul execută funcția *teletype output*: afișează pe ecran caracterul aflat în <code>AL</code> și mută automat cursorul, la fel cum s-ar întâmpla într-un terminal obișnuit.

Pentru a afișa un șir de caractere întreg, trebuie parcursă litera cu literă și apelată <code>int 0x10</code> pentru fiecare, oprindu-ne când întâlnim un octet <code>0</code> (terminatorul șirului, aceeași convenție folosită în limbajul de programare C).

### Exemplu: hello.asm

~~~asm
[org 0x7c00]
[bits 16]

start:
    xor ax, ax
    mov ds, ax
    mov si, msg

print_char:
    lodsb
    cmp al, 0
    je halt
    mov ah, 0x0e
    int 0x10
    jmp print_char

halt:
    jmp halt

msg db 'Hello, World!', 0

times 510-($-$$) db 0
dw 0xaa55
~~~

### Explicație pas cu pas

- <code>xor ax, ax</code> + <code>mov ds, ax</code> pun 0 în registrul <code>DS</code>. Unele BIOS-uri și emulatoare nu garantează valoarea inițială a lui <code>DS</code>, așa că o forțăm explicit la 0 (același segment folosit de adresa *0x7C00*).
- <code>mov si, msg</code> pune în <code>SI</code> adresa șirului de afișat.
- <code>lodsb</code> citește octetul de la adresa <code>DS:SI</code> în <code>AL</code> și incrementează automat <code>SI</code>.
- <code>cmp al, 0</code> + <code>je halt</code> verifică dacă am ajuns la terminator.
- <code>mov ah, 0x0e</code> + <code>int 0x10</code> afișează caracterul curent.
- Eticheta <code>halt</code> conține o buclă infinită. Fără ea, procesorul ar continua să execute octeții următori din memorie (care nu sunt cod valid).

### Rulare

~~~bash
nasm -f bin hello.asm -o hello.bin
qemu-system-x86_64 -fda hello.bin
~~~

Pe ecran va apărea **"Hello, World!"** în colțul din stânga sus. Acesta este deja un bootloader Hello World complet funcțional, scris în întregime în real mode.`,
		},
		{
			"real-mode-si-protected-mode",
			"Real mode și protected mode",
			"Limitările real mode-ului, structura GDT și pașii prin care procesorul trece în protected mode.",
			`Bootloaderul din pagina anterioară funcționează, dar **real mode** are limitări serioase:

- adresează cel mult 1 MB de memorie (adesea 640KB, restul de 360KB necesitând niște "artificii" complicate)
- nu oferă nicio protecție între segmentele de memorie (orice program care se blochează, va îngheța întregul sistem)
- nu poate folosi toate instrucțiunile pe 32 de biți ale procesorului

Orice sistem de operare modern are nevoie de **protected mode**, introdus odată cu procesorul Intel 80286 și extins la 32 de biți pe 80386.

În protected mode, adresele de memorie nu mai sunt calculate din perechi *[segment:offset]*. Procesorul folosește o structură numită **GDT** (*Global Descriptor Table*) pentru a descrie segmentele de memorie disponibile: unde încep, cât sunt de mari și ce tip de acces permit (cod sau date).

Trecerea la protected mode presupune trei lucruri:

1. construirea unui GDT valid
2. încărcarea lui în procesor cu instrucțiunea <code>lgdt</code>
3. setarea bitului PE (*Protection Enable*) din registrul de control <code>CR0</code>

### Definiția GDT-ului

~~~asm
gdt_start:
    dq 0x0                 ; descriptor nul (obligatoriu)

gdt_code:
    dw 0xffff              ; limită
    dw 0x0                 ; bază (biți 0-15)
    db 0x0                 ; bază (biți 16-23)
    db 10011010b           ; access byte
    db 11001111b           ; flags + limită (biți 16-19)
    db 0x0                 ; bază (biți 24-31)

gdt_data:
    dw 0xffff
    dw 0x0
    db 0x0
    db 10010010b
    db 11001111b
    db 0x0

gdt_end:

gdt_descriptor:
    dw gdt_end - gdt_start - 1
    dd gdt_start

CODE_SEG equ gdt_code - gdt_start
DATA_SEG equ gdt_data - gdt_start
~~~

- Primul descriptor (<code>dq 0x0</code>) este obligatoriu nul.
- <code>gdt_code</code> descrie un segment de cod: limită 0xFFFF (extinsă mai târziu la 4 GB), bază 0, access <code>10011010b</code> (prezent, ring 0, cod, executabil + citibil), flags <code>11001111b</code> (granularitate 4 KB + mod 32 biți).
- <code>gdt_data</code> este aproape identic, dar cu access <code>10010010b</code> (segment de date, scriere permisă).
- <code>CODE_SEG</code> și <code>DATA_SEG</code> sunt constante calculate la asamblare, folosite mai târziu ca selectori.

### Trecerea efectivă în protected mode

~~~asm
switch_to_pm:
    cli                     ; dezactivăm întreruperile
    lgdt [gdt_descriptor]   ; încărcăm GDT-ul
    mov eax, cr0
    or eax, 1               ; setăm bitul PE
    mov cr0, eax
    jmp CODE_SEG:init_pm    ; far jump obligatoriu

[bits 32]
init_pm:
    mov ax, DATA_SEG
    mov ds, ax
    mov ss, ax
    mov es, ax
    mov fs, ax
    mov gs, ax
    mov esp, 0x90000        ; stivă sub 1 MB
~~~

- <code>cli</code> dezactivează întreruperile hardware (cele din real mode nu mai sunt valide).
- Far jump-ul (<code>jmp CODE_SEG:init_pm</code>) golește coada de preîncărcare a procesorului.
- De la <code>init_pm</code> în jos, codul rulează pe 32 de biți.

În acest punct procesorul rulează deja în protected mode, dar ecranul e tot gol. Pasul următor arată cum se scrie pe ecran în noul mod.`,
		},
		{
			"hello-world-in-protected-mode",
			"Hello, World! în protected mode",
			"Scrierea directă în memoria video și bootloaderul final, complet, care afișează Hello, World! după trecerea în protected mode.",
			`În real mode, scrierea pe ecran s-a făcut prin BIOS (<code>int 0x10</code>). În protected mode, întreruperile BIOS nu mai sunt disponibile (le-am dezactivat cu <code>cli</code>), iar oricum BIOS-ul rulează cod pe 16 biți, incompatibil cu modul curent.

Soluția: scriem **direct în memoria video**.

Placa video mapează în memoria calculatorului, începând de la adresa <code>0xB8000</code>, o zonă specială numită *text mode buffer*. Fiecare caracter afișat pe ecran ocupă **2 octeți** consecutivi:

1. codul ASCII al caracterului
2. un octet de atribute (culoare text + fundal)

Pentru text alb pe fundal negru, valoarea standard este <code>0x0F</code>.

### Funcția de afișare

~~~asm
VIDEO_MEMORY   equ 0xb8000
WHITE_ON_BLACK equ 0x0f

print_string_pm:
    pusha
    mov edx, VIDEO_MEMORY

print_string_pm_loop:
    mov al, [ebx]           ; caracterul curent
    mov ah, WHITE_ON_BLACK  ; atribut culoare
    cmp al, 0
    je print_string_pm_done
    mov [edx], ax           ; scriem caracter + atribut
    add ebx, 1
    add edx, 2
    jmp print_string_pm_loop

print_string_pm_done:
    popa
    ret
~~~

- <code>pusha</code> / <code>popa</code> salvează și restaurează registrele.
- <code>EDX</code> ține adresa curentă din memoria video.
- <code>EBX</code> conține adresa șirului de afișat (parametrul funcției).
- Scriem câte 2 octeți deodată (<code>mov [edx], ax</code>).

### Bootloaderul final complet

Punând cap la cap tot ce am învățat, rezultă un singur bootloader funcțional care pornește în real mode, trece în protected mode și afișează "Hello, World!".

~~~asm
[org 0x7c00]
[bits 16]

start:
    xor ax, ax
    mov ds, ax
    jmp switch_to_pm

; ---------- GDT ----------
gdt_start:
    dq 0x0

gdt_code:
    dw 0xffff
    dw 0x0
    db 0x0
    db 10011010b
    db 11001111b
    db 0x0

gdt_data:
    dw 0xffff
    dw 0x0
    db 0x0
    db 10010010b
    db 11001111b
    db 0x0

gdt_end:

gdt_descriptor:
    dw gdt_end - gdt_start - 1
    dd gdt_start

CODE_SEG equ gdt_code - gdt_start
DATA_SEG equ gdt_data - gdt_start

; ---------- Switch to protected mode ----------
switch_to_pm:
    cli
    lgdt [gdt_descriptor]
    mov eax, cr0
    or eax, 1
    mov cr0, eax
    jmp CODE_SEG:init_pm

[bits 32]
init_pm:
    mov ax, DATA_SEG
    mov ds, ax
    mov ss, ax
    mov es, ax
    mov fs, ax
    mov gs, ax
    mov esp, 0x90000

    mov ebx, msg
    call print_string_pm
    jmp $

; ---------- Print ----------
VIDEO_MEMORY   equ 0xb8000
WHITE_ON_BLACK equ 0x0f

print_string_pm:
    pusha
    mov edx, VIDEO_MEMORY
print_string_pm_loop:
    mov al, [ebx]
    mov ah, WHITE_ON_BLACK
    cmp al, 0
    je print_string_pm_done
    mov [edx], ax
    add ebx, 1
    add edx, 2
    jmp print_string_pm_loop
print_string_pm_done:
    popa
    ret

msg db 'Hello, World!', 0

times 510-($-$$) db 0
dw 0xaa55
~~~

### Rulare

~~~bash
nasm -f bin boot.asm -o boot.bin
qemu-system-x86_64 -fda boot.bin
~~~

La rulare, ecranul rămâne gol o fracțiune de secundă (cât durează trecerea în protected mode), apoi apare textul **"Hello, World!"** scris direct în memoria video, fără niciun ajutor din partea BIOS-ului.`,
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
			"Câți bytes are Master Boot Record-ul (MBR)?",
			"256 bytes", "512 bytes", "1024 bytes", "4096 bytes",
			"B",
			"MBR-ul ocupă exact un sector de disc: 512 bytes, dintre care ultimii 2 sunt semnătura 0x55AA.",
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
