package main

const settingsPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Codex DataProxy Settings</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f7f7f4;
      --surface: #ffffff;
      --surface-2: #f1f3f5;
      --line: #d9dde3;
      --text: #111318;
      --muted: #667085;
      --accent: #10a37f;
      --accent-soft: #e7f7f2;
      --blue: #2563eb;
      --blue-soft: #eaf1ff;
      --danger: #d92d20;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        linear-gradient(90deg, rgba(17, 19, 24, 0.035) 1px, transparent 1px) 0 0 / 48px 48px,
        linear-gradient(rgba(17, 19, 24, 0.026) 1px, transparent 1px) 0 0 / 48px 48px,
        var(--bg);
      color: var(--text);
      font-family: "Segoe UI", "Microsoft YaHei", Arial, sans-serif;
      letter-spacing: 0;
    }
    button, input { font: inherit; }
    .app { display: grid; grid-template-rows: 62px 1fr 38px; min-height: 100vh; }
    .topbar {
      display: flex; align-items: center; justify-content: space-between; gap: 20px;
      padding: 0 22px; border-bottom: 1px solid var(--line);
      background: rgba(247, 247, 244, 0.92); backdrop-filter: blur(16px);
    }
    .brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
    .mark {
      display: grid; place-items: center; width: 32px; height: 32px;
      border: 1px solid rgba(16, 163, 127, 0.25); border-radius: 8px;
      background: var(--accent-soft); color: var(--accent); font-weight: 750;
    }
    .title { display: grid; gap: 2px; }
    .title strong { font-size: 15px; font-weight: 680; }
    .title span { color: var(--muted); font-size: 12px; }
    .badge {
      display: inline-flex; align-items: center; gap: 7px; min-height: 30px;
      padding: 0 11px; border: 1px solid var(--line); border-radius: 999px;
      background: rgba(255, 255, 255, 0.78); color: var(--muted); font-size: 12px;
      white-space: nowrap;
    }
    .badge.green { border-color: rgba(16, 163, 127, 0.22); background: var(--accent-soft); color: #08765c; }
    .badge.blue { border-color: rgba(37, 99, 235, 0.18); background: var(--blue-soft); color: #1d4ed8; }
    .dot { width: 7px; height: 7px; border-radius: 50%; background: var(--muted); }
    .dot.good { background: var(--accent); box-shadow: 0 0 0 4px rgba(16, 163, 127, 0.12); }
    .main { display: grid; grid-template-columns: 330px minmax(0, 1fr); min-height: 0; }
    .sidebar {
      display: grid; grid-template-rows: auto 1fr; min-height: 0;
      border-right: 1px solid var(--line); background: rgba(255, 255, 255, 0.56);
      backdrop-filter: blur(14px);
    }
    .sidebar-head { display: grid; gap: 12px; padding: 16px; border-bottom: 1px solid #e7e9ee; }
    .section-title { display: flex; justify-content: space-between; color: var(--muted); font-size: 12px; text-transform: uppercase; }
    .provider-list { display: grid; align-content: start; gap: 8px; padding: 12px; overflow: auto; }
    .provider {
      display: grid; grid-template-columns: 18px minmax(0, 1fr); gap: 10px; align-items: start;
      padding: 13px 11px; border: 1px solid #e7e9ee; border-radius: 8px;
      background: rgba(255,255,255,.72); cursor: pointer;
    }
    .provider.active { border-color: rgba(16, 163, 127, 0.38); background: #f4fffb; }
    .provider strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; }
    .provider span { display: block; overflow: hidden; color: var(--muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
    .content { display: grid; grid-template-rows: auto 1fr; min-width: 0; min-height: 0; }
    .content-head {
      display: flex; align-items: center; justify-content: space-between; gap: 18px;
      padding: 20px 24px; border-bottom: 1px solid #e7e9ee; background: rgba(255,255,255,.48);
    }
    h1 { margin: 0; font-size: 20px; font-weight: 680; }
    p { margin: 4px 0 0; color: var(--muted); font-size: 13px; }
    .actions { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
    .btn {
      display: inline-flex; align-items: center; justify-content: center; min-height: 34px;
      padding: 0 12px; border: 1px solid var(--line); border-radius: 7px;
      background: #fff; color: var(--text); cursor: pointer; white-space: nowrap;
    }
    .btn.primary { border-color: rgba(16,163,127,.55); background: var(--accent); color: white; }
    .icon-btn {
      width: 36px; min-width: 36px; padding: 0;
    }
    .icon-btn svg {
      width: 16px; height: 16px; stroke: currentColor;
    }
    .workspace {
      display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 18px;
      min-height: 0; padding: 20px 24px; overflow: auto;
    }
    .stack { display: grid; gap: 18px; align-content: start; min-width: 0; }
    .panel {
      border: 1px solid var(--line); border-radius: 8px; background: rgba(255,255,255,.94);
      box-shadow: 0 20px 50px rgba(17,19,24,.08); min-width: 0;
    }
    .panel-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; border-bottom: 1px solid #e7e9ee; }
    .panel-header strong { font-size: 13px; font-weight: 680; }
    .form { display: grid; gap: 14px; padding: 16px; }
    .grid-2 { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
    .field { display: grid; gap: 7px; min-width: 0; }
    .field span:first-child { color: var(--muted); font-size: 12px; }
    .control-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; }
    input {
      width: 100%; height: 36px; min-width: 0; padding: 0 10px;
      border: 1px solid var(--line); border-radius: 7px; background: #fff; color: var(--text); outline: none;
    }
    .route-table { display: grid; gap: 8px; padding: 12px; }
    .route {
      display: grid; grid-template-columns: 150px minmax(0, 1fr) auto; gap: 12px; align-items: center;
      padding: 12px; border: 1px solid #e7e9ee; border-radius: 8px; background: #fff;
    }
    .route strong { font-size: 13px; font-weight: 680; }
    .route small { display: block; margin-top: 4px; color: var(--muted); font-size: 12px; }
    .model-line { overflow: hidden; color: var(--muted); font-family: Consolas, monospace; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
    .metric-list { display: grid; gap: 10px; padding: 14px; }
    .metric { display: grid; gap: 4px; padding-bottom: 10px; border-bottom: 1px solid #e7e9ee; }
    .metric:last-child { border-bottom: 0; padding-bottom: 0; }
    .metric span { color: var(--muted); font-size: 12px; }
    .metric strong { overflow: hidden; font-size: 13px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
    .notice { padding: 12px; border: 1px solid #f2d79b; border-radius: 8px; background: #fff7df; color: #7c4b00; font-size: 13px; line-height: 1.5; }
    .statusbar {
      display: flex; align-items: center; justify-content: space-between; gap: 18px;
      padding: 0 16px; border-top: 1px solid var(--line); background: rgba(255,255,255,.82);
      color: var(--muted); font-size: 12px; white-space: nowrap; overflow: hidden;
    }
    @media (max-width: 1050px) {
      .main, .workspace { grid-template-columns: 1fr; }
      .sidebar { border-right: 0; border-bottom: 1px solid var(--line); max-height: 300px; }
    }
  </style>
</head>
<body>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <div class="mark">DP</div>
        <div class="title">
          <strong>Codex DataProxy</strong>
          <span>本地代理、模型路由与中转站设置</span>
        </div>
      </div>
      <div class="actions">
        <span class="badge green"><span class="dot good"></span>Local only</span>
        <span class="badge blue" id="activeBadge">加载中</span>
      </div>
    </header>

    <main class="main">
      <aside class="sidebar">
        <div class="sidebar-head">
          <div class="section-title"><span>中转站</span><span id="providerCount">0 个 endpoint</span></div>
          <button class="btn primary" id="addProviderButton">新增中转站</button>
        </div>
        <div class="provider-list" id="providerList"></div>
      </aside>

      <section class="content">
        <div class="content-head">
          <div>
            <h1 id="providerName">Settings</h1>
            <p>models 可为空；为空时自动拉取 /v1/models。合并时后面的 Key 覆盖前面的 Key。</p>
          </div>
          <div class="actions">
            <button class="btn" id="refreshButton">刷新模型</button>
            <button class="btn" id="activateButton">设为当前</button>
            <button class="btn primary" id="saveButton">保存</button>
          </div>
        </div>

        <div class="workspace">
          <div class="stack">
            <section class="panel">
              <div class="panel-header">
                <strong>Endpoint</strong>
                <span class="badge green" id="endpointState">Active</span>
              </div>
              <div class="form">
                <div class="grid-2">
                  <label class="field"><span>名称</span><input id="nameInput"></label>
                  <label class="field"><span>ID</span><input id="idInput" readonly></label>
                </div>
                <label class="field"><span>Base URL</span><input id="baseInput"></label>
                <div class="grid-2">
                  <label class="field"><span>默认模型</span><input id="defaultModelInput"></label>
                  <label class="field"><span>排序</span><input id="sortInput"></label>
                </div>
                <label class="field"><span>代理地址</span><input id="proxyInput" readonly></label>
              </div>
            </section>

            <section class="panel">
              <div class="panel-header">
                <strong>Key 与模型路由</strong>
                <button class="btn" id="addKeyButton">新增 Key</button>
              </div>
              <div class="route-table">
                <div class="notice">当前版本先接入本地代理和配置读取；编辑、排序、删除会在下一阶段接入。</div>
                <div id="keyRoutes"></div>
              </div>
            </section>
          </div>

          <aside class="stack">
            <section class="panel">
              <div class="panel-header"><strong>路由状态</strong></div>
              <div class="metric-list">
                <div class="metric"><span>本地代理</span><strong id="proxyMetric">-</strong></div>
                <div class="metric"><span>当前 endpoint</span><strong id="providerMetric">-</strong></div>
                <div class="metric"><span>模型合集</span><strong id="modelMetric">-</strong></div>
                <div class="metric"><span>配置来源</span><strong>config/*.yaml</strong></div>
              </div>
            </section>

            <section class="panel">
              <div class="panel-header"><strong>模型合集</strong></div>
              <div class="metric-list" id="modelList"></div>
            </section>
          </aside>
        </div>
      </section>
    </main>

    <footer class="statusbar">
      <span>Settings: {{BASE_URL}}/settings</span>
      <span>/v1/responses -> active endpoint -> key matched by model</span>
    </footer>
  </div>

  <script>
    const api = path => path;
    const apiFetch = (path, options = {}) => {
      const headers = Object.assign({}, options.headers || {});
      return fetch(api(path), Object.assign({}, options, { headers, credentials: "same-origin" }));
    };
    let selectedId = "";
    let currentData = {};
    let currentProvider = {};

    async function loadProviders() {
      const response = await apiFetch("/api/providers");
      const data = await response.json();
      const providers = data.providers || [];
      const active = providers.find(p => p.active && p.enabled) || providers[0] || {};
      selectedId = selectedId || active.id || "";
      const selected = providers.find(p => p.id === selectedId) || active;
      currentData = data;

      document.getElementById("providerCount").textContent = providers.length + " 个 endpoint";
      document.getElementById("activeBadge").textContent = active.name ? "当前：" + active.name : "未配置";
      document.getElementById("providerList").innerHTML = providers.map(p =>
        '<button class="provider ' + (p.id === selected.id ? "active" : "") + '" onclick="selectProvider(' + "'" + escapeAttr(p.id) + "'" + ')">' +
          '<span class="dot ' + (p.active ? "good" : "") + '"></span>' +
          '<span><strong>' + escapeHtml(p.name || p.id) + '</strong><span>' + ((p.keys || []).length) + ' keys · ' + escapeHtml(p.default_model || "-") + '</span></span>' +
        '</button>'
      ).join("");

      renderProvider(selected, data);
    }

    function selectProvider(id) {
      selectedId = id;
      loadProviders();
    }

    function renderProvider(provider, data) {
      provider = provider || {};
      currentProvider = JSON.parse(JSON.stringify(provider));
      document.getElementById("providerName").textContent = provider.name || "Settings";
      document.getElementById("nameInput").value = provider.name || "";
      document.getElementById("idInput").value = provider.id || "";
      document.getElementById("baseInput").value = provider.base_url || "";
      document.getElementById("defaultModelInput").value = provider.default_model || "";
      document.getElementById("defaultModelInput").placeholder = "为空时使用模型合集里的第一个模型";
      document.getElementById("sortInput").value = provider.sort || 10;
      document.getElementById("proxyInput").value = data.proxy_url || "";
      document.getElementById("proxyMetric").textContent = data.proxy_url || "-";
      document.getElementById("providerMetric").textContent = provider.name || "-";
      document.getElementById("modelMetric").textContent = (data.models || []).length + " 个模型";

      document.getElementById("keyRoutes").innerHTML = (provider.keys || []).map((k, index) =>
        '<div class="route" data-key-index="' + index + '">' +
          '<div>' +
            '<input class="key-name" value="' + escapeAttr(k.name || "") + '" placeholder="Key 名称，例如 GPT 主 Key">' +
            '<small>' + (k.default ? "默认 Key · " : "") + (k.enabled ? "已启用" : "已停用") + '</small>' +
          '</div>' +
          '<div class="field">' +
            '<span class="control-row"><input class="key-api" type="password" value="' + escapeAttr(k.api_key || "") + '" placeholder="填写 API Key，例如 sk-..."><button class="btn icon-btn" type="button" onclick="toggleKeyVisibility(this)" aria-label="Show API key" title="Show API key">' + eyeIcon(false) + '</button></span>' +
            '<input class="key-models" value="' + escapeAttr(k.models || "") + '" placeholder="models 为空时使用 /v1/models 自动获取">' +
          '</div>' +
          '<div class="actions"><button class="btn" type="button" onclick="makeDefaultKey(' + index + ')">默认</button></div>' +
        '</div>'
      ).join("");

      document.getElementById("modelList").innerHTML = (data.models || []).slice(0, 24).map(m =>
        '<div class="metric"><span>' + escapeHtml(keyDisplayName(provider, data.routes && data.routes[m] && data.routes[m].key_id)) + '</span><strong>' + escapeHtml(m) + '</strong></div>'
      ).join("") || '<div class="metric"><span>暂无模型</span><strong>点击刷新模型</strong></div>';
    }

    function collectProvider() {
      const provider = JSON.parse(JSON.stringify(currentProvider || {}));
      provider.name = document.getElementById("nameInput").value.trim();
      provider.id = document.getElementById("idInput").value.trim();
      provider.base_url = document.getElementById("baseInput").value.trim();
      provider.default_model = document.getElementById("defaultModelInput").value.trim();
      provider.sort = Number(document.getElementById("sortInput").value || 10);
      provider.enabled = provider.enabled !== false;
      provider.keys = Array.from(document.querySelectorAll("[data-key-index]")).map((row, index) => {
        const old = (currentProvider.keys || [])[Number(row.dataset.keyIndex)] || {};
        return {
          id: old.id || ("key-" + (index + 1)),
          name: row.querySelector(".key-name").value.trim() || old.name || ("Key " + (index + 1)),
          api_key: row.querySelector(".key-api").value.trim(),
          enabled: old.enabled !== false,
          sort: old.sort || ((index + 1) * 10),
          default: !!old.default,
          models: row.querySelector(".key-models").value.trim()
        };
      });
      if (!provider.keys.some(k => k.default) && provider.keys.length > 0) {
        provider.keys[0].default = true;
      }
      return provider;
    }

    async function saveProvider() {
      const provider = collectProvider();
      const button = document.getElementById("saveButton");
      button.disabled = true;
      button.textContent = "保存中";
      try {
        const response = await apiFetch("/api/providers", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(provider)
        });
        if (!response.ok) throw new Error(await response.text());
        selectedId = provider.id;
        await loadProviders();
      } catch (error) {
        alert("保存失败：" + error.message);
      } finally {
        button.disabled = false;
        button.textContent = "保存";
      }
    }

    async function activateProvider() {
      await saveProvider();
      const provider = collectProvider();
      const response = await apiFetch("/api/providers/activate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: provider.id })
      });
      if (!response.ok) {
        alert("切换失败：" + await response.text());
        return;
      }
      selectedId = provider.id;
      await loadProviders();
    }

    function addKey() {
      currentProvider.keys = currentProvider.keys || [];
      currentProvider.keys.push({
        id: "key-" + (currentProvider.keys.length + 1),
        name: "",
        api_key: "",
        enabled: true,
        sort: (currentProvider.keys.length + 1) * 10,
        default: currentProvider.keys.length === 0,
        models: ""
      });
      renderProvider(currentProvider, currentData);
    }

    function addProvider() {
      const count = ((currentData && currentData.providers) || []).length + 1;
      selectedId = "provider-" + count;
      const provider = {
        id: selectedId,
        name: "新中转站",
        active: false,
        enabled: true,
        sort: count * 10,
        base_url: "",
        default_model: "",
        keys: [{
          id: "default",
          name: "",
          api_key: "",
          enabled: true,
          sort: 10,
          default: true,
          models: ""
        }]
      };
      currentData.providers = currentData.providers || [];
      renderProvider(provider, currentData);
    }

    function makeDefaultKey(index) {
      currentProvider.keys = (currentProvider.keys || []).map((key, keyIndex) => {
        key.default = keyIndex === index;
        return key;
      });
      renderProvider(currentProvider, currentData);
    }

    function toggleKeyVisibility(button) {
      const input = button.parentElement.querySelector(".key-api");
      const visible = input.type === "text";
      const nextVisible = !visible;
      input.type = nextVisible ? "text" : "password";
      button.innerHTML = eyeIcon(nextVisible);
      button.setAttribute("aria-label", nextVisible ? "Hide API key" : "Show API key");
      button.setAttribute("title", nextVisible ? "Hide API key" : "Show API key");
    }

    function eyeIcon(visible) {
      if (visible) {
        return '<svg viewBox="0 0 24 24" fill="none" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 3l18 18"></path><path d="M10.6 10.6a2 2 0 0 0 2.8 2.8"></path><path d="M9.9 4.2A10.8 10.8 0 0 1 12 4c5 0 8.5 3.5 10 8a13.5 13.5 0 0 1-3.1 4.8"></path><path d="M6.6 6.6A13.3 13.3 0 0 0 2 12c1.5 4.5 5 8 10 8 1.4 0 2.7-.3 3.9-.8"></path></svg>';
      }
      return '<svg viewBox="0 0 24 24" fill="none" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"></path><circle cx="12" cy="12" r="3"></circle></svg>';
    }

    function keyDisplayName(provider, keyId) {
      const key = ((provider && provider.keys) || []).find(item => item.id === keyId);
      return (key && (key.name || key.id)) || keyId || "key";
    }

    async function refreshModels() {
      const button = document.getElementById("refreshButton");
      button.disabled = true;
      button.textContent = "刷新中";
      try {
        await apiFetch("/api/models/refresh", { method: "POST" });
        await loadProviders();
      } finally {
        button.disabled = false;
        button.textContent = "刷新模型";
      }
    }

    function escapeHtml(value) {
      return String(value || "").replace(/[&<>"']/g, ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]));
    }

    function escapeAttr(value) {
      return String(value || "").replace(/['\\]/g, "\\$&");
    }

    document.getElementById("refreshButton").addEventListener("click", refreshModels);
    document.getElementById("saveButton").addEventListener("click", saveProvider);
    document.getElementById("activateButton").addEventListener("click", activateProvider);
    document.getElementById("addKeyButton").addEventListener("click", addKey);
    document.getElementById("addProviderButton").addEventListener("click", addProvider);
    loadProviders();
  </script>
</body>
</html>`
