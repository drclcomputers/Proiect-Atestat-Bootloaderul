package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const maxAsmSource = 64 * 1024

func playgroundPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "playground", PageData{Title: "Playground"})
}

func nasmStatusHandler(w http.ResponseWriter, r *http.Request) {
	nasm, err := findNasm()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      false,
			"os":      runtime.GOOS,
			"arch":    runtime.GOARCH,
			"message": missingNasmHelp(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
		"path": nasm,
	})
}

func assembleHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAsmSource)
	src, err := readLimitedBody(r)
	if err != nil {
		http.Error(w, "sursa e prea mare sau invalidă (max 64 KB)", http.StatusBadRequest)
		return
	}
	if len(bytes.TrimSpace(src)) == 0 {
		http.Error(w, "lipsește sursa Assembly", http.StatusBadRequest)
		return
	}

	nasm, err := findNasm()
	if err != nil {
		http.Error(w, missingNasmHelp(), http.StatusServiceUnavailable)
		return
	}

	bin, logOut, err := assembleNASM(nasm, src)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write(logOut)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Boot-Size", strconv.Itoa(len(bin)))
	w.Header().Set("X-Nasm-Path", nasm)
	_, _ = w.Write(bin)
}

func missingNasmHelp() string {
	return "Niciun NASM care chiar rulează pe " + runtime.GOOS + "/" + runtime.GOARCH + ".\n\n" +
		"Serverul ignoră binarele pentru alt OS (ex. tools/nasm.exe pe Mac).\n\n" +
		"macOS:  brew install nasm\n" +
		"Linux:  sudo apt-get install nasm   (sau tools/nasm ELF)\n" +
		"Windows (fără instalare):  tools/nasm.exe  — binarul oficial win64"
}

func findNasm() (string, error) {
	var candidates []string
	add := func(p string) {
		if p == "" {
			return
		}
		for _, c := range candidates {
			if c == p {
				return
			}
		}
		candidates = append(candidates, p)
	}

	for _, name := range []string{"nasm", "nasm.exe"} {
		if p, err := exec.LookPath(name); err == nil {
			add(p)
		}
	}

	var dirs []string
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(wd, "tools"), wd)
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Dir(exe)
		dirs = append(dirs, filepath.Join(d, "tools"), d)
	}
	for _, dir := range dirs {
		add(filepath.Join(dir, "nasm"))
		add(filepath.Join(dir, "nasm.exe"))
	}

	var last error
	for _, p := range candidates {
		if err := probeNasm(p); err != nil {
			last = err
			continue
		}
		return p, nil
	}
	if last != nil {
		return "", last
	}
	return "", errors.New("nasm not found")
}

func probeNasm(path string) error {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return errors.New("missing")
	}
	if err := compatibleBinary(path); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-v")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func compatibleBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var magic [4]byte
	n, _ := f.Read(magic[:])
	if n < 2 {
		return errors.New("binar gol")
	}

	isPE := magic[0] == 'M' && magic[1] == 'Z'
	isELF := n >= 4 && magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F'
	isMachO := n >= 4 && (bytes.Equal(magic[:], []byte{0xfe, 0xed, 0xfa, 0xce}) ||
		bytes.Equal(magic[:], []byte{0xce, 0xfa, 0xed, 0xfe}) ||
		bytes.Equal(magic[:], []byte{0xfe, 0xed, 0xfa, 0xcf}) ||
		bytes.Equal(magic[:], []byte{0xcf, 0xfa, 0xed, 0xfe}) ||
		bytes.Equal(magic[:], []byte{0xca, 0xfe, 0xba, 0xbe}))

	switch runtime.GOOS {
	case "windows":
		if isELF || isMachO {
			return errors.New("binar unix, nu rulează pe Windows")
		}
	default:
		if isPE {
			return errors.New("nasm.exe (Windows) ignorat pe " + runtime.GOOS)
		}
	}
	return nil
}

func readLimitedBody(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}

func assembleNASM(nasm string, src []byte) ([]byte, []byte, error) {
	dir, err := os.MkdirTemp("", "atestat-asm-*")
	if err != nil {
		return nil, []byte("nu am putut crea director temporar"), err
	}
	defer os.RemoveAll(dir)

	asmPath := filepath.Join(dir, "boot.asm")
	binPath := filepath.Join(dir, "boot.bin")
	if err := os.WriteFile(asmPath, src, 0o600); err != nil {
		return nil, []byte("nu am putut scrie boot.asm"), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, nasm, "-f", "bin", "-o", binPath, asmPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, []byte("nasm a durat prea mult (timeout 5s)"), errors.New("timeout")
	}
	if err != nil {
		msg := bytes.TrimSpace(out)
		if len(msg) == 0 {
			msg = []byte(err.Error())
		}
		return nil, msg, err
	}

	bin, err := os.ReadFile(binPath)
	if err != nil {
		return nil, []byte("nasm a rulat, dar boot.bin lipsește"), err
	}
	if len(bin) == 0 {
		return nil, []byte("binarul e gol"), errors.New("empty")
	}
	if len(bin) > 1474560 {
		return nil, []byte("binarul depășește o dischetă 1.44 MB"), errors.New("too large")
	}
	return bin, out, nil
}
