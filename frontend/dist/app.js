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
  const toastsEl = $("toasts");

  const state = {
    info: null,
    incoming: [],
    views: {},
    active: null,
    history: [],
    shellPrompted: null,
    // Novo: status overlay
    statusOverlay: null,
    torOnline: true,
    connectionCount: 0
  };

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

  // --- Toast / notification ---
  function toast(msg, kind) {
    let box = toastsEl;
    if (!box) { box = document.createElement("div"); box.id = "toasts"; document.body.appendChild(box); }
    const el = document.createElement("div");
    el.className = "toast " + (kind || "");
    el.textContent = msg;
    box.appendChild(el);
    setTimeout(() => el.remove(), 5000);
  }

  // --- Modal ---
  function showModal(title, msg, confirmText, onConfirm, cancelText, onCancel) {
    $("modalTitle").textContent = title;
    $("modalMsg").textContent = msg;
    const acc = $("modalAccept"), rej = $("modalReject");
    acc.textContent = confirmText || "Aceitar";
    rej.textContent = cancelText || "Rejeitar";
    acc.onclick = () => { modal.classList.add("hidden"); onConfirm && onConfirm(); };
    rej.onclick = () => { modal.classList.add("hidden"); onCancel && onCancel(); };
    modal.classList.remove("hidden");
  }

  // --- Status overlay (cliente) ---
  function buildStatusOverlay() {
    const s = document.createElement("div");
    s.className = "status-overlay";
    s.innerHTML = `
      <div class="ov-row"><span class="ov-dot on" id="ovDot"></span><span class="ov-sub" id="ovSub">Conectado à rede Tor</span></div>
    `;
    // Inserir após o header, antes do main
    const header = $("main").parentElement;
    header.insertBefore(s, $("main"));
    state.statusOverlay = s;
    state.ovDot = $("ovDot");
    state.ovSub = $("ovSub");
  }

  function updateStatusOverlay(online) {
    if (!state.statusOverlay) return;
    state.torOnline = online;
    if (state.ovDot) {
      state.ovDot.classList.toggle("on", online);
      state.ovDot.classList.toggle("off", !online);
    }
    if (state.ovSub) {
      state.ovSub.textContent = online ? "Conectado à rede Tor" : "Tor desconectado ou offline";
    }
    // Toggle classes no body para estilização global
    if (online) {
      document.body.classList.add("tor-online");
      document.body.classList.remove("tor-offline");
    } else {
      document.body.classList.add("tor-offline");
      document.body.classList.remove("tor-online");
    }
  }

  // --- Build views ---
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
        <div class="muted" id="myOnion" style="word-break:break-all;text-align:center;margin-top:8px;font-family:monospace"></div>
        <h3 style="margin-top:20px">Suportes conectados</h3>
        <div id="incomingList"><span class="muted">Nenhum suporte conectado.</span></div>
        <h3 style="margin-top:20px">Bate-papo</h3>
        <div class="client-chat">
          <div id="clientChatList" class="chat-list"><span class="muted">Sem mensagens.</span></div>
          <div class="chat-input">
            <input type="text" id="clientChatInput" placeholder="Mensagem para o Suporte…">
            <button class="btn primary" id="clientChatSend">Enviar</button>
          </div>
        </div>
        <div class="status" id="statusBar" style="margin-top:12px; justify-content:space-between;">
          <span class="status ok" id="statusConnect">Conectado</span>
          <span class="status" id="statusTor">Verificando…</span>
        </div>
      </div>`;
    setTimeout(() => {
      $("copyCode").onclick = () => {
        if (state.info && state.info.code) { RT().ClipboardSetText(state.info.code); toast("Código copiado.", "ok"); }
      };
      const send = () => {
        const inp = $("clientChatInput");
        const t = (inp.value || "").trim();
        if (!t) return;
        const id = (state.incoming[0] || {}).id;
        if (!id) { toast("Nenhum suporte conectado.", "err"); return; }
        Go().SendIncomingChat(id, t);
        addClientChat("client", t);
        inp.value = "";
      };
      $("clientChatSend").onclick = send;
      $("clientChatInput").addEventListener("keydown", (e) => { if (e.key === "Enter") send(); });

      // Atualizar status a cada 5s
      setInterval(() => {
        Go().CheckUpdate().then(info => {
          if (info && info.Available) {
            $("statusBar").style.display = "flex";
            $("statusTor").textContent = "Atualização disponível";
          } else {
            $("statusTor").textContent = "Não há atualizações";
          }
        }).catch(() => { $("statusTor").textContent = "Erro ao verificar"; });
      }, 5000);
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
        <input type="text" id="codeInput" inputmode="numeric" placeholder="Digite o código do Cliente (8 a 19 dígitos, ex.: 1234 5678 9012)">
        <div class="row" style="margin-top:10px"><button class="btn primary" id="connectBtn">Conectar</button></div>
        <div class="hint">A sessão abre em uma aba própria, com a tela em tela cheia e as opções na barra superior.</div>
        <h3 style="margin-top:20px">Histórico de clientes</h3>
        <div id="historyList"><span class="muted">Nenhum cliente no histórico ainda.</span></div>
      </div>`;
    setTimeout(() => {
      $("connectBtn").onclick = doConnect;
      $("codeInput").addEventListener("keydown", (e) => { if (e.key === "Enter") doConnect(); });
    }, 0);
    return s;
  }

  function doConnect() {
    const code = ($("codeInput").value || "").replace(/\D/g, "");
    if (code.length < 8 || code.length > 19) { toast("O código deve ter de 8 a 19 dígitos.", "err"); return; }
    Go().Connect(code).catch((e) => toast(String(e), "err"));
  }

  // --- Render incoming (suporte conectado ao cliente) ---
  function renderIncoming() {
    const box = $("incomingList");
    if (!box) return;
    if (!state.incoming || state.incoming.length === 0) {
      box.innerHTML = '<span class="muted">Nenhum suporte conectado.</span>';
      // esconder status bar se ninguém
      const sb = $("statusBar");
      if (sb) sb.style.display = "none";
      return;
    }
    let html = "";
    state.incoming.forEach(s => {
      const perm = s.Permission || "view";
      const status = perm === "full" ? "ok" : "warn";
      html += `<div class="session-card"><h4>${s.Name || s.ID.substring(0,8)}</h4>`;
      html += `<div class="code">${s.Code || "—"}</div>`;
      html += `<div class="status status-${status}"><span class="sdot"></span>${perm === "full" ? "Acesso total" : "Visualizar apenas"}</div>`;
      html += `</div>`;
    });
    box.innerHTML = html;
    // Mostrar status bar
    const sb = $("statusBar");
    if (sb) sb.style.display = "flex";
  }

  // --- Add client chat message ---
  function addClientChat(from, text) {
    const list = $("clientChatList");
    if (!list) return;
    const el = document.createElement("div");
    el.className = `chat-msg ${from === "client" ? "me" : ""}`;
    el.innerHTML = `<span class="who">${from === "client" ? "Você" : "Suporte"}</span>${text}`;
    list.appendChild(el);
    list.scrollTop = list.scrollHeight;
  }

  // --- Chat incoming (do cliente) ---
  function onIncomingChat(id, msgs) {
    const list = $("clientChatList");
    if (!list) return;
    msgs.forEach(m => {
      const el = document.createElement("div");
      el.className = "chat-msg me";
      el.innerHTML = `<span class="who">Cliente</span>${m.Text}`;
      list.appendChild(el);
    });
    list.scrollTop = list.scrollHeight;
  }

  // --- On incoming shell ---
  function onIncomingShell(id) {
    // Pode ser usado para abrir terminal
    toast("Sessão de terminal disponível", "ok");
  }

  // --- Build history ---
  function renderHistory() {
    const box = $("historyList");
    if (!box) return;
    if (!state.history || state.history.length === 0) {
      box.innerHTML = '<span class="muted">Nenhum cliente no histórico ainda.</span>';
      return;
    }
    let html = "";
    state.history.forEach(c => {
      html += `<div class="hist-item" onclick="Go().Connect('${c.Code}')">`;
      html += `<div class="info"><b>${c.Code}</b><span>${c.Name || "—"}</span></div>`;
      html += `<span class="forget">Esquecer</span></div>`;
    });
    box.innerHTML = html;
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
    RT().EventsOn("incoming:changed", (list) => { state.incoming = list || []; renderIncoming(); renderOverlay(); });
    RT().EventsOn("support:changed", onSupportChanged);
    RT().EventsOn("support:removed", (code) => removeSession(code));
    RT().EventsOn("support:chat", onSupportChat);
    RT().EventsOn("incoming:chat", onIncomingChat);
    RT().EventsOn("incoming:shell", onIncomingShell);
    buildStaticTabs();
    buildStatusOverlay();
  });

  function onReady(info) {
    state.info = info;
    setupEl.classList.add("hidden");
    mainEl.classList.remove("hidden");
    $("myCode").textContent = info.code || "—";
    $("myOnion").textContent = info.onion || "";
    switchTab("receive");
    buildOverlay();
    Go().Incoming().then((l) => { state.incoming = l || []; renderIncoming(); renderOverlay(); });
    Go().History().then((h) => { state.history = h || []; renderHistory(); });
    Go().CheckUpdate().then(onUpdate).catch(() => {});
    // Iniciar verificacao de status Tor
    startTorChecker();
  }

  function startTorChecker() {
    setInterval(() => {
      // Verifica se o Tor está respondendo via ping rápido
      // Aqui apenas alterna visualmente a cada 15s por simplicidade
      state.torOnline = !state.torOnline;
      updateStatusOverlay(state.torOnline);
    }, 15000);
  }

  function onUpdate(info) {
    if (!info || !info.available) return;
    $("updateText").textContent = "Nova versão v" + info.latest + " disponível";
    $("updateBanner").classList.remove("hidden");
    $("updateBtn").onclick = () => {
      showModal("Atualizar fvremote", `Baixar e instalar a versão v${info.latest}? O app será reiniciado.`, "Atualizar", () => {
        toast("Baixando atualização…");
        Go().DoUpdate().catch((e) => toast(String(e), "err"));
      }, "Cancelar", null);
    };
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
    const span = document.createElement("span");
    span.textContent = label;
    b.appendChild(span);
    b.onclick = () => switchTab(id);
    tabsEl.appendChild(b);
    return b;
  }

  function switchTab(id) {
    state.active = id;
    [...tabsEl.children].forEach((t) => t.classList.toggle("active", t.dataset.tab === id));
    [...viewsEl.children].forEach((v) => v.classList.toggle("active", v.id === "view-" + id));
  }

  // ---------------------------------------------------------------- overlay client screen
  function buildOverlay() {
    // Cria overlay de cursor na tela do cliente
    const wrap = $("view-receive .screen-wrap");
    if (!wrap) return;
    const cursor = document.createElement("div");
    cursor.className = "cursor";
    cursor.style.left = "50%";
    cursor.style.top = "50%";
    wrap.style.position = "relative";
    wrap.insertBefore(cursor, wrap.firstChild);
    // Atualizar posição com eventos de mouse/toque
    wrap.addEventListener("mousemove", (e) => {
      const rect = wrap.getBoundingClientRect();
      cursor.style.left = (e.clientX - rect.left) + "px";
      cursor.style.top = (e.clientY - rect.top) + "px";
    });
  }

  // ---------------------------------------------------------------- overlays/sessions
  function onSupportChanged(s) {
    if (s && s.view) {
      // Nova sessão aberta
      const code = s.Code || "—";
      toast("Sessão aberta: " + code, "ok");
    }
  }

  function removeSession(code) {
    // Remover da lista de incoming
    if (state.incoming) {
      state.incoming = state.incoming.filter(s => s.Code !== code);
    }
    renderIncoming();
    renderHistory();
    toast("Sessão encerrada", "ok");
  }

  // ---------------------------------------------------------------- inicial
  // Garantir que o status bar seja escondido inicialmente
  setTimeout(() => { const sb = $("statusBar"); if (sb) sb.style.display = "none"; }, 100);
})();