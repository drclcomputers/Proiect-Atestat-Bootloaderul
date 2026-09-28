(function () {
    const V86_WASM = "/static/vendor/v86/v86.wasm";
    const BIOS = "/static/vendor/v86/seabios.bin";
    const VGA_BIOS = "/static/vendor/v86/vgabios.bin";
    const FLOPPY_SIZE = 1474560; // 1.44 MB

    const templates = {
        hello: `[org 0x7c00]
[bits 16]

start:
    xor ax, ax
    mov ds, ax
    mov si, msg

print_char:
    lodsb
    cmp al, 0
    je hang
    mov ah, 0x0e
    int 0x10
    jmp print_char

hang:
    cli
    hlt
    jmp hang

msg db 'Hello, World!', 0

times 510-($-$$) db 0
dw 0xaa55
`,
        start: `[org 0x7c00]
[bits 16]

start:
    jmp start

times 510-($-$$) db 0
dw 0xaa55
`,
        pm: `[org 0x7c00]
[bits 16]

start:
    xor ax, ax
    mov ds, ax
    jmp switch_to_pm

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

VIDEO_MEMORY   equ 0xb8000
WHITE_ON_BLACK equ 0x0f

print_string_pm:
    pusha
    mov edx, VIDEO_MEMORY
.loop:
    mov al, [ebx]
    mov ah, WHITE_ON_BLACK
    cmp al, 0
    je .done
    mov [edx], ax
    add ebx, 1
    add edx, 2
    jmp .loop
.done:
    popa
    ret

msg db 'Hello, World!', 0

times 510-($-$$) db 0
dw 0xaa55
`,
    };

    const $ = (id) => document.getElementById(id);
    const logs = $("console-logs");
    const editor = $("assembly-code");
    const statusEl = $("emulator-status");
    const placeholder = $("screen-placeholder");
    const stats = $("code-stats");
    const restartBtn = $("restart-vm-btn");
    const stopBtn = $("stop-vm-btn");

    let emulator = null;
    let lastBin = null;
    let modeTimer = null;

    function flagOn(v) {
        if (v == null) return false;
        if (typeof v === "number") return v !== 0;
        if (typeof v === "boolean") return v;
        if (v.length != null) return v[0] !== 0;
        return false;
    }

    function cpuMode(emu) {
        try {
            const cpu = emu && emu.v86 && emu.v86.cpu;
            if (!cpu) return null;
            const cr0 = cpu.cr ? cpu.cr[0] : 0;
            const pe = flagOn(cpu.protected_mode) || cr0 & 1;
            const pg = (cr0 >>> 31) & 1;
            if (pg) return "protected mode + paging";
            if (pe) return "protected mode";
            return "real mode";
        } catch (e) {
            return null;
        }
    }

    function watchCpuMode(emu) {
        if (modeTimer) {
            clearInterval(modeTimer);
            modeTimer = null;
        }
        let last = null;
        let ticks = 0;
        modeTimer = setInterval(function () {
            ticks++;
            if (!emu || emu !== emulator) {
                clearInterval(modeTimer);
                modeTimer = null;
                return;
            }
            const mode = cpuMode(emu);
            if (mode && mode !== last) {
                log("v86: CPU în " + mode, "ok");
                setStatus(mode);
                last = mode;
            }
            if (ticks >= 40) {
                clearInterval(modeTimer);
                modeTimer = null;
            }
        }, 100);
    }

    function log(msg, kind) {
        const line = document.createElement("div");
        line.className = kind ? "log-" + kind : "";
        line.textContent = msg;
        logs.appendChild(line);
        logs.scrollTop = logs.scrollHeight;
    }

    function setStatus(text) {
        statusEl.textContent = text;
    }

    function padFloppy(boot) {
        const img = new Uint8Array(FLOPPY_SIZE);
        img.set(boot.subarray(0, Math.min(boot.length, FLOPPY_SIZE)));
        return img;
    }

    async function assemble() {
        log("nasm -f bin boot.asm -o boot.bin");
        const res = await fetch("/playground/assemble", {
            method: "POST",
            headers: { "Content-Type": "text/plain; charset=utf-8" },
            body: editor.value,
        });

        if (res.status === 401) {
            location.href = "/login?next=/playground";
            throw new Error("not logged in");
        }

        const buf = await res.arrayBuffer();
        if (!res.ok) {
            const err = new TextDecoder().decode(buf);
            log(err || "eroare la asamblare", "err");
            throw new Error("assemble failed");
        }
        lastBin = new Uint8Array(buf);
        const nasmPath = res.headers.get("X-Nasm-Path") || "nasm";
        stats.textContent = lastBin.length + " octeți (nasm)";
        log("NASM: " + nasmPath, "ok");
        if (lastBin.length < 512) {
            log(
                "atenție: binarul are " +
                    lastBin.length +
                    " octeți (un MBR are 512)",
                "warn",
            );
        }
        const sig =
            lastBin.length >= 512
                ? lastBin[510].toString(16).padStart(2, "0") +
                  lastBin[511].toString(16).padStart(2, "0")
                : "—";
        log(
            "ok, " + lastBin.length + " octeți, octeții 510–511 = " + sig,
            "ok",
        );
        return lastBin;
    }

    async function boot(bin) {
        if (modeTimer) {
            clearInterval(modeTimer);
            modeTimer = null;
        }
        if (emulator) {
            try {
                emulator.destroy();
            } catch (e) {}
            emulator = null;
        }
        placeholder.style.display = "none";
        setStatus("pornește…");

        const floppy = padFloppy(bin);
        emulator = new V86({
            wasm_path: V86_WASM,
            memory_size: 32 * 1024 * 1024,
            vga_memory_size: 2 * 1024 * 1024,
            screen_container: $("screen_container"),
            bios: { url: BIOS },
            vga_bios: { url: VGA_BIOS },
            fda: { buffer: floppy.buffer },
            boot_order: 0x321,
            fastboot: true,
            disable_speaker: true,
            autostart: true,
        });

        emulator.add_listener("emulator-loaded", function () {
            log("v86: BIOS încărcat", "ok");
            restartBtn.disabled = false;
            stopBtn.disabled = false;
            watchCpuMode(emulator);
        });
    }

    $("template-select").addEventListener("change", function (e) {
        editor.value = templates[e.target.value] || templates.hello;
        log("șablon: " + e.target.value);
    });

    $("copy-code").addEventListener("click", function () {
        navigator.clipboard.writeText(editor.value).then(function () {
            log("sursă copiată", "ok");
        });
    });

    $("clear-logs").addEventListener("click", function () {
        logs.textContent = "";
    });

    $("build-boot-btn").addEventListener("click", async function () {
        try {
            const bin = await assemble();
            await boot(bin);
        } catch (err) {
            setStatus("eroare");
            placeholder.style.display = "";
        }
    });

    $("download-bin-btn").addEventListener("click", async function () {
        try {
            const bin = lastBin || (await assemble());
            const blob = new Blob([bin], { type: "application/octet-stream" });
            const a = document.createElement("a");
            a.href = URL.createObjectURL(blob);
            a.download = "boot.bin";
            a.click();
            URL.revokeObjectURL(a.href);
        } catch (e) {
            /* logged */
        }
    });

    restartBtn.addEventListener("click", function () {
        if (emulator) {
            emulator.restart();
            log("reboot", "warn");
        }
    });

    stopBtn.addEventListener("click", function () {
        if (modeTimer) {
            clearInterval(modeTimer);
            modeTimer = null;
        }
        if (emulator) {
            try {
                emulator.stop();
                emulator.destroy();
            } catch (e) {
                /* ignore */
            }
            emulator = null;
        }
        setStatus("oprit");
        placeholder.style.display = "";
        restartBtn.disabled = true;
        stopBtn.disabled = true;
        log("VM oprită", "warn");
    });

    editor.value = templates.hello;
    fetch("/playground/nasm")
        .then(function (r) {
            return r.json();
        })
        .then(function (info) {
            if (info.ok) {
                log(
                    "NASM gata pe " +
                        info.os +
                        "/" +
                        info.arch +
                        ": " +
                        info.path,
                    "ok",
                );
            } else {
                log(
                    "NASM indisponibil pe " + info.os + "/" + info.arch,
                    "warn",
                );
                log(info.message || "", "warn");
            }
        })
        .catch(function () {
            log("playground gata. NASM se verifică la asamblare.", "info");
        });
})();
