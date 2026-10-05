"use strict";

(function () {
  const $ = (id) => document.getElementById(id);
  const setupEl = $("setup");
  const mainEl = $("main");
  const setupBar = $("setupBar");
  const setupMsg = $("setupMsg");
  const tabsEl = $("tabs");
  const viewsEl = $("views");
  const modal = $("modal");

  const state = { info: null, incoming: [], views: {}, active: null };
  const special = {
    Enter: "enter", Backspace: "backspace", Tab: "tab", Escape: "esc",
    Delete: "delete", Insert: "insert", ArrowUp: "up", ArrowDown: "down",
    ArrowLeft: "left", ArrowRight: "right", Home: "home", End: "end",
    PageUp: "pageup", PageDown: "pagedown", " ": "space",
    Shift: "shift", Control: "ctrl", Alt: "alt", Meta: "cmd",
    F1: "f1", F2: "f2", F3: "f3", F4: "f4", F5: "f5", F6: "f6",
    F7: "f7", F8: "f8", F9: "f9", F10: "f10", F11: "f11", F12: "f12",
  };

  function Go() { return window.go.main.App; }
  function RT() { return window.runtime; }

  function whenReady(fn) {
    if (window.runtime && window.go && window.go.main && window.go.main.App) { fn(); return; }
    setTimeout(() => whenReady(fn), 50);
  }

  function toast(msg, kind) {
    let box = $("toasts");
    if (!box) { box = document.createElement("div"); box.id = "toasts"; document.body.appendChild(box); }
    const el = document.createElement("div");
    el.className = "toast " + (kind || "");
    el.textContent = msg;
    box.appendChild(el);
    setTimeout(() => el.remove(), 4000);
  }

  // ---------------------------------------------------------------- boot
  whenReady(function () {
    RT().EventsOn("setup", (d) => {
      if (!d) return;
      setupBar.style.width = (d.pct || 0) + "%";
      setupMsg.textContent = d.msg || "";
    });
    RT().EventsOn("fatal", (msg) => { setupMsg.textContent = "Erro: " + msg; toast(msg, "err"); });
    RT().EventsOn("ready", onReady);
    RT().EventsOn("incoming:request", onIncomingRequest);
    RT().EventsOn("incoming:changed", (list) => { state.incoming = list || []; renderIncoming(); });
    RT().EventsOn("support:changed", onSupportChanged);
    RT().EventsOn("support:removed", (code) => removeSession(code));
    buildStaticTabs();
  });

  function onReady(info) {
    state.info = info;
    setupEl.classList.add("hidden");
    mainEl.classList.remove("hidden");
    $("myCode").textContent = info.code || "—";
    $("myOnion").textContent = info.onion || "";
    switchTab("receive");
    Go().Incoming().then((list) => { state.incoming = list || []; renderIncoming(); });
  }

  // ---------------------------------------------------------------- tabs
  function buildStaticTabs() {
    tabsEl.innerHTML = "";
    addTab("receive", "Receber suporte");
    addTab("support", "Prestar suporte");
    viewsEl.innerHTML = "";
    viewsEl.appendChild(buildReceiveView());
    viewsEl.appendChild(buildSupportView());
  }

  function addTab(id, label) {
    const b = document.createElement("button");
    b.className = "tab";
    b.dataset.tab = id;
    b.textContent = label;
    b.onclick = () => switchTab(id);
    tabsEl.appendChild(b);
    return b;
  }

  function switchTab(id) {
    state.active = id;
    [...tabsEl.children].forEach((t) => t.classList.toggle("active", t.dataset.tab === id));
    [...viewsEl.children].forEach((v) => v.classList.toggle("active", v.id === "view-" + id));
    if (id && id.startsWith("sess:")) {
      const v = state.views[id.slice(5)];
      if (v && v.img && v.permission === "full") v.img.focus();
    }
  }

  function buildReceiveView() {
    const s = document.createElement("section");
    s.className = "view";
    s.id = "view-receive";
    s.innerHTML = `
      <div class="pane">
        <h2 style="text-align:center">Seu código único</h2>
        <div class="code" id="myCode">—</div>
        <div class="row" style="justify-content:center"><button class="btn primary" id="copyCode">Copiar código</button></div>
        <p class="hint" style="text-align:center">Compartilhe este código com quem vai prestar o suporte. Ele encontra você na rede Tor, sem servidor central.</p>
        <div class="muted" id="myOnion" style="word-break:break-all;text-align:center;margin-top:10px;font-family:monospace"></div>
        <h3 style="margin-top:28px">Suportes conectados</h3>
        <div id="incomingList"><span class="muted">Nenhum suporte conectado.</span></div>
      </div>`;
    setTimeout(() => {
      $("copyCode").onclick = () => {
        if (state.info && state.info.code) { RT().ClipboardSetText(state.info.code); toast("Código copiado.", "ok"); }
      };
    }, 0);
    return s;
  }

  function buildSupportView() {
    const s = document.createElement("section");
    s.className = "view";
    s.id = "view-support";
    s.innerHTML = `
      <div class="pane">
        <h2>Prestar suporte</h2>
        <input type="text" id="codeInput" inputmode="numeric" placeholder="Digite os 19 dígitos do código do Cliente">
        <div class="row" style="margin-top:12px"><button class="btn primary" id="connectBtn">Conectar</button></div>
        <div class="hint">A sessão abre em uma aba própria, com a tela em tela cheia e as opções na barra superior.</div>
      </div>`;
    setTimeout(() => {
      $("connectBtn").onclick = doConnect;
      $("codeInput").addEventListener("keydown", (e) => { if (e.key === "Enter") doConnect(); });
    }, 0);
    return s;
  }

  function doConnect() {
    const code = ($("codeInput").value || "").replace(/\D/g, "");
    if (code.length !== 19) { toast("O código deve ter 19 dígitos.", "err"); return; }
    Go().Connect(code).catch((e) => toast(String(e), "err"));
  }

  // ---------------------------------------------------------------- incoming (eu sendo assistido)
  function renderIncoming() {
    const box = $("incomingList");
    if (!box) return;
    if (!state.incoming || state.incoming.length === 0) {
      box.innerHTML = '<span class="muted">Nenhum suporte conectado.</span>';
      return;
    }
    box.innerHTML = "";
    state.incoming.forEach((s) => {
      const card = document.createElement("div");
      card.className = "session-card";
      const full = s.permission === "full";
      card.innerHTML = `<h4>${esc(s.name)} (${esc(s.host)})</h4>
        <div class="muted">Acesso: ${full ? "controle total" : "somente leitura"}</div>`;
      const row = document.createElement("div");
      row.className = "row";
      row.style.marginTop = "8px";
      const toggle = document.createElement("button");
      toggle.className = "btn";
      toggle.textContent = full ? "Voltar para somente leitura" : "Dar acesso total";
      toggle.onclick = () => Go().SetIncomingPermission(s.id, full ? "view" : "full");
      const disc = document.createElement("button");
      disc.className = "btn ghost";
      disc.textContent = "Desconectar";
      disc.onclick = () => Go().DisconnectIncoming(s.id);
      row.appendChild(toggle); row.appendChild(disc);
      card.appendChild(row);
      box.appendChild(card);
    });
  }

  function onIncomingRequest(s) {
    $("modalMsg").textContent = `${s.name} (${s.host}) quer se conectar ao seu computador. O acesso inicia como somente leitura.`;
    modal.classList.remove("hidden");
    $("modalAccept").onclick = () => { modal.classList.add("hidden"); Go().AcceptIncoming(s.id); toast("Suporte conectado — somente leitura.", "ok"); };
    $("modalReject").onclick = () => { modal.classList.add("hidden"); Go().RejectIncoming(s.id); };
  }

  // ---------------------------------------------------------------- support sessions (eu assistindo)
  function onSupportChanged(v) {
    let view = state.views[v.code];
    if (!view) view = createSessionView(v);
    updateSession(view, v);
  }

  function onSupportRemoved(code) { removeSession(code); }

  function createSessionView(v) {
    const code = v.code;
    const section = document.createElement("section");
    section.className = "view";
    section.id = "view-sess:" + code;
    section.innerHTML = `
      <div class="topbar">
        <span class="client"></span>
        <span class="sessstatus"></span>
        <div class="spacer"></div>
        <button class="btn cap" disabled>Capturar mouse</button>
        <button class="btn files">Arquivos</button>
        <button class="btn fs">Tela cheia</button>
        <button class="btn ghost disc">Desconectar</button>
      </div>
      <div class="screen-wrap">
        <img class="screen connecting" alt="">
      </div>`;
    viewsEl.appendChild(section);

    const view = {
      code, section,
      client: section.querySelector(".client"),
      status: section.querySelector(".sessstatus"),
      img: section.querySelector(".screen"),
      capBtn: section.querySelector(".cap"),
      filesBtn: section.querySelector(".files"),
      fsBtn: section.querySelector(".fs"),
      discBtn: section.querySelector(".disc"),
      wrap: section.querySelector(".screen-wrap"),
      permission: "view",
      accepted: false,
      capture: false,
      vx: 0, vy: 0,
      remoteW: 2, remoteH: 2,
      files: null,
    };
    state.views[code] = view;

    view.capBtn.onclick = () => toggleCapture(view);
    view.fsBtn.onclick = () => Go().ToggleFullscreen();
    view.filesBtn.onclick = () => openFiles(view);
    view.discBtn.onclick = () => Go().Disconnect(code);
    wireInput(view);

    const tab = addTab("sess:" + code, "Cliente: " + (v.clientName || code));
    tab.dataset.code = code;
    return view;
  }

  function updateSession(view, v) {
    view.remoteW = v.remoteW || view.remoteW;
    view.remoteH = v.remoteH || view.remoteH;
    view.client.textContent = "Cliente: " + (v.clientName || v.code) + (v.clientHost ? " (" + v.clientHost + ")" : "");
    const tab = [...tabsEl.children].find((t) => t.dataset.tab === "sess:" + v.code);
    if (tab) tab.firstChild.textContent = "Cliente: " + (v.clientName || v.code);

    view.status.textContent = v.message || v.state;
    view.status.className = "sessstatus " + stateClass(v.state);

    const full = v.permission === "full";
    view.permission = v.permission;
    view.capBtn.disabled = !full;

    if (v.state === "accepted" && !view.accepted) {
      view.accepted = true;
      view.img.classList.remove("connecting");
      Go().ViewURL(v.code).then((url) => { if (url) view.img.src = url; });
      switchTab("sess:" + v.code);
    }
    if (v.state === "error" || v.state === "rejected") {
      view.img.classList.add("connecting");
    }
  }

  function stateClass(s) {
    if (s === "accepted") return "ok";
    if (s === "error" || s === "rejected") return "err";
    if (s === "waiting") return "warn";
    return "";
  }

  function removeSession(code) {
    const view = state.views[code];
    if (!view) return;
    document.exitPointerLock && document.exitPointerLock();
    view.section.remove();
    const tab = [...tabsEl.children].find((t) => t.dataset.tab === "sess:" + code);
    if (tab) tab.remove();
    delete state.views[code];
    if (state.active === "sess:" + code) switchTab("support");
  }

  // ---------------------------------------------------------------- mouse / keyboard
  function wireInput(view) {
    const img = view.img;

    img.addEventListener("mousemove", (e) => {
      if (view.permission !== "full") return;
      if (document.pointerLockElement === img) {
        const scale = view.remoteW / Math.max(1, img.clientWidth);
        view.vx = clamp(view.vx + e.movementX * scale, 0, view.remoteW - 1);
        view.vy = clamp(view.vy + e.movementY * scale, 0, view.remoteH - 1);
        Go().Move(view.code, Math.round(view.vx), Math.round(view.vy));
      } else {
        const p = toRemote(view, e);
        Go().Move(view.code, p.x, p.y);
      }
    });

    img.addEventListener("mousedown", (e) => {
      if (view.permission !== "full") return;
      e.preventDefault();
      const p = toRemote(view, e);
      Go().Move(view.code, p.x, p.y);
      Go().Click(view.code, e.button === 0, true);
    });
    img.addEventListener("mouseup", (e) => {
      if (view.permission !== "full") return;
      Go().Click(view.code, e.button === 0, false);
    });
    img.addEventListener("contextmenu", (e) => { e.preventDefault(); });
    img.addEventListener("wheel", (e) => {
      if (view.permission !== "full") return;
      e.preventDefault();
      Go().Scroll(view.code, e.deltaY < 0 ? 3 : -3);
    }, { passive: false });
  }

  function toRemote(view, e) {
    const img = view.img;
    const r = img.getBoundingClientRect();
    const rw = view.remoteW, rh = view.remoteH;
    const scale = Math.min(r.width / rw, r.height / rh);
    const dispW = rw * scale, dispH = rh * scale;
    const offX = (r.width - dispW) / 2, offY = (r.height - dispH) / 2;
    let x = (e.clientX - r.left - offX) / scale;
    let y = (e.clientY - r.top - offY) / scale;
    return { x: clamp(Math.round(x), 0, rw - 1), y: clamp(Math.round(y), 0, rh - 1) };
  }

  function toggleCapture(view) {
    if (view.permission !== "full") return;
    view.capture = !view.capture;
    view.capBtn.textContent = view.capture ? "Soltar mouse" : "Capturar mouse";
    view.capBtn.classList.toggle("active", view.capture);
    view.img.classList.toggle("capture", view.capture);
    if (view.capture) {
      view.vx = Math.round(view.remoteW / 2);
      view.vy = Math.round(view.remoteH / 2);
      if (view.img.requestPointerLock) {
        const p = view.img.requestPointerLock();
        if (p && p.catch) p.catch(() => {});
      }
    } else {
      if (document.exitPointerLock) document.exitPointerLock();
    }
  }

  document.addEventListener("keydown", (e) => {
    const view = activeSession();
    if (!view || view.permission !== "full") return;
    if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) return;
    const name = special[e.key];
    if (name) { e.preventDefault(); Go().Key(view.code, name, true); }
    else if (e.key.length === 1) { e.preventDefault(); Go().Key(view.code, e.key, true); }
  });
  document.addEventListener("keyup", (e) => {
    const view = activeSession();
    if (!view || view.permission !== "full") return;
    if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) return;
    const name = special[e.key];
    if (name) Go().Key(view.code, name, false);
    else if (e.key.length === 1) Go().Key(view.code, e.key, false);
  });

  function activeSession() {
    if (!state.active || !state.active.startsWith("sess:")) return null;
    return state.views[state.active.slice(5)] || null;
  }

  // ---------------------------------------------------------------- files
  function openFiles(view) {
    if (!view.files) view.files = createFilesPanel(view);
    view.files.root.classList.toggle("hidden");
    if (!view.files.root.classList.contains("hidden")) loadFiles(view, "");
  }

  function createFilesPanel(view) {
    const root = document.createElement("div");
    root.className = "files-panel hidden";
    root.innerHTML = `
      <div class="fp-head">
        <button class="btn up">↑</button>
        <button class="btn refresh">Atualizar</button>
        <button class="btn upload">Enviar</button>
        <button class="btn download">Baixar</button>
        <div class="spacer"></div>
        <button class="btn ghost close">✕</button>
      </div>
      <div class="fp-path"></div>
      <div class="fp-list"></div>`;
    view.wrap.appendChild(root);
    const panel = { root, path: "", list: root.querySelector(".fp-list"), pathLb: root.querySelector(".fp-path"), items: [], sel: null };
    root.querySelector(".close").onclick = () => root.classList.add("hidden");
    root.querySelector(".refresh").onclick = () => loadFiles(view, panel.path);
    root.querySelector(".up").onclick = () => { const p = panel.path.replace(/\/[^/]*\/?$/, "") || "/"; loadFiles(view, p); };
    root.querySelector(".upload").onclick = () => Go().UploadFile(view.code).then(() => loadFiles(view, panel.path)).catch((e) => toast(String(e), "err"));
    root.querySelector(".download").onclick = () => {
      if (panel.sel) Go().DownloadFile(view.code, panel.sel).catch((e) => toast(String(e), "err"));
      else toast("Selecione um arquivo.", "err");
    };
    return panel;
  }

  function loadFiles(view, path) {
    Go().ListFiles(view.code, path).then((items) => {
      const panel = view.files;
      panel.path = path || (items.length ? "" : path);
      panel.items = items || [];
      panel.sel = null;
      panel.list.innerHTML = "";
      (items || []).forEach((it) => {
        const row = document.createElement("div");
        row.className = "fp-item" + (it.dir ? " dir" : "");
        row.innerHTML = `<span>${it.dir ? "📁 " : ""}${esc(it.name)}</span><span class="sz">${it.dir ? "" : human(it.size)}</span>`;
        row.onclick = () => {
          if (it.dir) { loadFiles(view, it.path); }
          else {
            [...panel.list.children].forEach((c) => c.classList.remove("sel"));
            row.classList.add("sel");
            panel.sel = it.path;
            panel.pathLb.textContent = it.path;
          }
        };
        panel.list.appendChild(row);
      });
    }).catch((e) => toast(String(e), "err"));
  }

  // ---------------------------------------------------------------- utils
  function clamp(v, a, b) { return Math.max(a, Math.min(b, v)); }
  function esc(s) { return String(s == null ? "" : s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c])); }
  function human(n) { if (n < 1024) return n + " B"; if (n < 1048576) return (n / 1024).toFixed(1) + " KB"; if (n < 1073741824) return (n / 1048576).toFixed(1) + " MB"; return (n / 1073741824).toFixed(1) + " GB"; }
})();
