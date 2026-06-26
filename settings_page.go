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
      --bg: #f7f8f5;
      --surface: #ffffff;
      --surface-soft: #f1f4f2;
      --surface-blue: #f0f6ff;
      --line: #d9dfdc;
      --line-strong: #c5cdc9;
      --text: #101214;
      --muted: #66736f;
      --muted-2: #87918d;
      --accent: #10a37f;
      --accent-strong: #087c62;
      --accent-soft: #e8f7f2;
      --blue: #2563eb;
      --blue-soft: #eaf1ff;
      --warning: #b7791f;
      --warning-soft: #fff7e6;
      --danger: #d92d20;
      --danger-soft: #fff0ee;
      --shadow: 0 20px 55px rgba(16, 18, 20, 0.08);
    }
    * { box-sizing: border-box; }
    html, body { min-height: 100%; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        linear-gradient(90deg, rgba(16, 18, 20, 0.035) 1px, transparent 1px) 0 0 / 42px 42px,
        linear-gradient(rgba(16, 18, 20, 0.025) 1px, transparent 1px) 0 0 / 42px 42px,
        var(--bg);
      color: var(--text);
      font-family: "Segoe UI", "Microsoft YaHei", Arial, sans-serif;
      letter-spacing: 0;
    }
    button, input {
      font: inherit;
    }
    button {
      border: 0;
    }
    .app {
      display: grid;
      grid-template-rows: 64px minmax(0, 1fr) 38px;
      min-height: 100vh;
    }
    .topbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      padding: 0 22px;
      border-bottom: 1px solid var(--line);
      background: rgba(247, 248, 245, 0.9);
      backdrop-filter: blur(18px);
    }
    .brand {
      display: flex;
      align-items: center;
      min-width: 0;
      gap: 12px;
    }
    .brand-mark {
      display: grid;
      place-items: center;
      width: 34px;
      height: 34px;
      border: 1px solid rgba(16, 163, 127, 0.24);
      border-radius: 8px;
      background: #ffffff;
      color: var(--accent-strong);
      font-weight: 760;
      box-shadow: 0 8px 25px rgba(16, 163, 127, 0.11);
    }
    .brand-text {
      min-width: 0;
      display: grid;
      gap: 2px;
    }
    .brand-text strong {
      overflow: hidden;
      color: var(--text);
      font-size: 15px;
      font-weight: 720;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .brand-text span {
      overflow: hidden;
      color: var(--muted);
      font-size: 12px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .top-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 8px;
      min-width: 0;
      flex-wrap: wrap;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 7px;
      min-height: 30px;
      max-width: 280px;
      padding: 0 10px;
      border: 1px solid var(--line);
      border-radius: 999px;
      background: rgba(255, 255, 255, 0.78);
      color: var(--muted);
      font-size: 12px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .badge.green {
      border-color: rgba(16, 163, 127, 0.25);
      background: var(--accent-soft);
      color: var(--accent-strong);
    }
    .badge.blue {
      border-color: rgba(37, 99, 235, 0.18);
      background: var(--blue-soft);
      color: #1d4ed8;
    }
    .badge.warn {
      border-color: #ecd59d;
      background: var(--warning-soft);
      color: #7a4b05;
    }
    .dot {
      width: 7px;
      height: 7px;
      border-radius: 50%;
      background: var(--muted-2);
      flex: 0 0 auto;
    }
    .dot.good {
      background: var(--accent);
      box-shadow: 0 0 0 4px rgba(16, 163, 127, 0.13);
    }
    .dot.warn {
      background: var(--warning);
      box-shadow: 0 0 0 4px rgba(183, 121, 31, 0.13);
    }
    .layout {
      display: grid;
      grid-template-columns: 300px minmax(0, 1fr) 320px;
      min-height: 0;
    }
    .providers-pane {
      display: grid;
      grid-template-rows: auto minmax(0, 1fr);
      min-width: 0;
      min-height: 0;
      border-right: 1px solid var(--line);
      background: rgba(255, 255, 255, 0.58);
      backdrop-filter: blur(14px);
    }
    .pane-head {
      display: grid;
      gap: 12px;
      padding: 16px;
      border-bottom: 1px solid rgba(217, 223, 220, 0.8);
    }
    .section-line {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      color: var(--muted);
      font-size: 12px;
    }
    .provider-list {
      display: grid;
      align-content: start;
      gap: 8px;
      min-height: 0;
      padding: 12px;
      overflow: auto;
    }
    .provider-item {
      display: grid;
      grid-template-columns: 18px minmax(0, 1fr);
      gap: 10px;
      width: 100%;
      min-height: 74px;
      padding: 12px;
      border: 1px solid rgba(217, 223, 220, 0.9);
      border-radius: 8px;
      background: rgba(255, 255, 255, 0.78);
      color: var(--text);
      cursor: pointer;
      text-align: left;
    }
    .provider-item:hover {
      border-color: var(--line-strong);
      background: #ffffff;
    }
    .provider-item.selected {
      border-color: rgba(16, 163, 127, 0.42);
      background: #f4fffb;
      box-shadow: 0 8px 28px rgba(16, 163, 127, 0.09);
    }
    .provider-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
      min-width: 0;
      gap: 8px;
    }
    .provider-title strong {
      overflow: hidden;
      font-size: 13px;
      font-weight: 690;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .provider-meta {
      display: grid;
      gap: 4px;
      margin-top: 5px;
      color: var(--muted);
      font-size: 12px;
    }
    .provider-meta span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .editor {
      display: grid;
      grid-template-rows: auto minmax(0, 1fr);
      min-width: 0;
      min-height: 0;
    }
    .editor-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      min-width: 0;
      padding: 20px 24px;
      border-bottom: 1px solid rgba(217, 223, 220, 0.8);
      background: rgba(255, 255, 255, 0.46);
    }
    .headline {
      min-width: 0;
      display: grid;
      gap: 5px;
    }
    h1 {
      margin: 0;
      overflow: hidden;
      font-size: 20px;
      font-weight: 720;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .subline {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      color: var(--muted);
      font-size: 12px;
    }
    .subline span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 8px;
      flex-wrap: wrap;
    }
    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 7px;
      min-height: 34px;
      max-width: 220px;
      padding: 0 12px;
      border: 1px solid var(--line);
      border-radius: 7px;
      background: #ffffff;
      color: var(--text);
      cursor: pointer;
      font-size: 13px;
      white-space: nowrap;
    }
    .btn:hover {
      border-color: var(--line-strong);
      background: #fbfcfb;
    }
    .btn.primary {
      border-color: rgba(16, 163, 127, 0.4);
      background: var(--accent);
      color: #ffffff;
    }
    .btn.primary:hover {
      background: #0e9273;
    }
    .btn.blue {
      border-color: rgba(37, 99, 235, 0.3);
      background: var(--blue);
      color: #ffffff;
    }
    .btn.danger {
      border-color: rgba(217, 45, 32, 0.24);
      background: var(--danger-soft);
      color: #a91f14;
    }
    .btn.ghost {
      background: transparent;
    }
    .btn.icon {
      width: 34px;
      min-width: 34px;
      padding: 0;
    }
    .btn:disabled {
      opacity: 0.58;
      cursor: default;
    }
    .workspace {
      min-width: 0;
      min-height: 0;
      padding: 20px 24px 24px;
      overflow: auto;
    }
    .stack {
      display: grid;
      gap: 16px;
      align-content: start;
      min-width: 0;
    }
    .surface {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: rgba(255, 255, 255, 0.94);
      box-shadow: var(--shadow);
    }
    .surface-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      min-height: 52px;
      padding: 13px 15px;
      border-bottom: 1px solid rgba(217, 223, 220, 0.78);
    }
    .surface-head strong {
      font-size: 13px;
      font-weight: 710;
    }
    .form-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 14px;
      padding: 16px;
    }
    .field {
      display: grid;
      gap: 7px;
      min-width: 0;
    }
    .field.full {
      grid-column: 1 / -1;
    }
    .field label,
    .field .label {
      color: var(--muted);
      font-size: 12px;
    }
    input[type="text"],
    input[type="password"],
    input[type="number"] {
      width: 100%;
      height: 36px;
      min-width: 0;
      padding: 0 10px;
      border: 1px solid var(--line);
      border-radius: 7px;
      background: #ffffff;
      color: var(--text);
      outline: none;
    }
    input[type="text"]:focus,
    input[type="password"]:focus,
    input[type="number"]:focus {
      border-color: rgba(16, 163, 127, 0.55);
      box-shadow: 0 0 0 3px rgba(16, 163, 127, 0.12);
    }
    input[readonly] {
      background: var(--surface-soft);
      color: var(--muted);
    }
    .toggle-row {
      display: flex;
      align-items: center;
      gap: 10px;
      min-height: 36px;
      color: var(--text);
      font-size: 13px;
    }
    .toggle-row input {
      width: 16px;
      height: 16px;
      accent-color: var(--accent);
      flex: 0 0 auto;
    }
    .key-list {
      display: grid;
      gap: 10px;
      padding: 12px;
    }
    .key-row {
      display: grid;
      grid-template-columns: minmax(150px, 0.8fr) minmax(190px, 1fr) minmax(190px, 1fr) 144px;
      gap: 10px;
      align-items: end;
      min-width: 0;
      padding: 12px;
      border: 1px solid rgba(217, 223, 220, 0.86);
      border-radius: 8px;
      background: #ffffff;
    }
    .key-tools {
      display: grid;
      grid-template-columns: repeat(4, 34px);
      gap: 6px;
      justify-content: end;
    }
    .secret-field {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 34px;
      gap: 6px;
    }
    .compact-controls {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 8px;
      margin-top: 8px;
    }
    .rail {
      display: grid;
      align-content: start;
      gap: 16px;
      min-width: 0;
      min-height: 0;
      padding: 20px 18px;
      border-left: 1px solid var(--line);
      background: rgba(255, 255, 255, 0.55);
      overflow: auto;
    }
    .metric-list {
      display: grid;
      gap: 0;
      padding: 6px 14px 14px;
    }
    .metric {
      display: grid;
      grid-template-columns: minmax(92px, 0.8fr) minmax(0, 1.2fr);
      gap: 10px;
      min-height: 42px;
      align-items: center;
      border-bottom: 1px solid rgba(217, 223, 220, 0.72);
    }
    .metric:last-child {
      border-bottom: 0;
    }
    .metric span {
      color: var(--muted);
      font-size: 12px;
    }
    .metric strong {
      overflow: hidden;
      color: var(--text);
      font-size: 12px;
      font-weight: 650;
      text-align: right;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .model-tools {
      display: grid;
      gap: 10px;
      padding: 14px 14px 4px;
    }
    .model-list {
      display: grid;
      gap: 8px;
      max-height: 420px;
      padding: 10px 14px 14px;
      overflow: auto;
    }
    .model-item {
      display: grid;
      gap: 4px;
      min-width: 0;
      padding: 9px 10px;
      border: 1px solid rgba(217, 223, 220, 0.78);
      border-radius: 8px;
      background: #ffffff;
    }
    .model-item strong {
      overflow: hidden;
      font-family: Consolas, "SFMono-Regular", monospace;
      font-size: 12px;
      font-weight: 650;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .model-item span {
      overflow: hidden;
      color: var(--muted);
      font-size: 12px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .empty {
      padding: 18px;
      color: var(--muted);
      font-size: 13px;
      text-align: center;
    }
    .statusbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      min-width: 0;
      padding: 0 16px;
      border-top: 1px solid var(--line);
      background: rgba(255, 255, 255, 0.82);
      color: var(--muted);
      font-size: 12px;
      white-space: nowrap;
    }
    .statusbar span {
      overflow: hidden;
      text-overflow: ellipsis;
    }
    @media (max-width: 1200px) {
      .layout {
        grid-template-columns: 280px minmax(0, 1fr);
      }
      .rail {
        grid-column: 1 / -1;
        border-left: 0;
        border-top: 1px solid var(--line);
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }
    @media (max-width: 860px) {
      .app {
        grid-template-rows: auto minmax(0, 1fr) auto;
      }
      .topbar,
      .editor-head {
        align-items: stretch;
        flex-direction: column;
        padding: 14px 16px;
      }
      .layout,
      .rail {
        grid-template-columns: 1fr;
      }
      .providers-pane {
        border-right: 0;
        border-bottom: 1px solid var(--line);
        max-height: 330px;
      }
      .workspace {
        padding: 16px;
      }
      .form-grid,
      .key-row {
        grid-template-columns: 1fr;
      }
      .key-tools {
        justify-content: start;
      }
      .statusbar {
        align-items: flex-start;
        flex-direction: column;
        padding: 8px 14px;
      }
    }
  </style>
</head>
<body>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <div class="brand-mark">DP</div>
        <div class="brand-text">
          <strong>Codex DataProxy</strong>
          <span>本地代理控制台</span>
        </div>
      </div>
      <div class="top-actions">
        <span class="badge green"><span class="dot good"></span>Local only</span>
        <span class="badge blue" id="activeBadge">加载中</span>
      </div>
    </header>

    <main class="layout">
      <aside class="providers-pane">
        <div class="pane-head">
          <div class="section-line">
            <span>中转站</span>
            <span id="providerCount">0</span>
          </div>
          <button class="btn primary" type="button" id="addProviderButton">新增中转站</button>
        </div>
        <div class="provider-list" id="providerList"></div>
      </aside>

      <section class="editor">
        <div class="editor-head">
          <div class="headline">
            <h1 id="providerName">Settings</h1>
            <div class="subline">
              <span class="badge" id="endpointState">未配置</span>
              <span id="endpointSummary">-</span>
            </div>
          </div>
          <div class="actions">
            <button class="btn" type="button" id="refreshButton">刷新模型</button>
            <button class="btn blue" type="button" id="activateButton">设为当前</button>
            <button class="btn primary" type="button" id="saveButton">保存</button>
          </div>
        </div>

        <div class="workspace">
          <div class="stack">
            <section class="surface">
              <div class="surface-head">
                <strong>Endpoint</strong>
                <span class="badge" id="endpointBadge">Draft</span>
              </div>
              <div class="form-grid">
                <div class="field">
                  <label for="nameInput">名称</label>
                  <input id="nameInput" type="text" placeholder="例如 DataProxy" />
                </div>
                <div class="field">
                  <label for="idInput">ID</label>
                  <input id="idInput" type="text" readonly />
                </div>
                <div class="field full">
                  <label for="baseInput">Base URL</label>
                  <input id="baseInput" type="text" placeholder="https://example.com/v1" />
                </div>
                <div class="field">
                  <label for="defaultModelInput">默认模型</label>
                  <input id="defaultModelInput" type="text" placeholder="留空时使用模型合集第一项" />
                </div>
                <div class="field">
                  <label for="sortInput">排序</label>
                  <input id="sortInput" type="number" min="0" step="10" />
                </div>
                <div class="field">
                  <span class="label">状态</span>
                  <label class="toggle-row"><input id="enabledInput" type="checkbox" /> 启用中转站</label>
                </div>
                <div class="field">
                  <label for="proxyInput">本地代理</label>
                  <input id="proxyInput" type="text" readonly />
                </div>
              </div>
            </section>

            <section class="surface">
              <div class="surface-head">
                <strong>Key 路由</strong>
                <div class="actions">
                  <button class="btn" type="button" id="addKeyButton">新增 Key</button>
                </div>
              </div>
              <div class="key-list" id="keyRoutes"></div>
            </section>
          </div>
        </div>
      </section>

      <aside class="rail">
        <section class="surface">
          <div class="surface-head"><strong>运行状态</strong></div>
          <div class="metric-list">
            <div class="metric"><span>本地代理</span><strong id="proxyMetric">-</strong></div>
            <div class="metric"><span>App Server</span><strong id="appServerMetric">-</strong></div>
            <div class="metric"><span>Token 文件</span><strong id="appServerTokenMetric">-</strong></div>
            <div class="metric"><span>当前中转站</span><strong id="providerMetric">-</strong></div>
            <div class="metric"><span>默认模型</span><strong id="defaultModelMetric">-</strong></div>
            <div class="metric"><span>模型合集</span><strong id="modelMetric">-</strong></div>
            <div class="metric"><span>配置来源</span><strong>config/*.yaml</strong></div>
          </div>
        </section>

        <section class="surface">
          <div class="surface-head">
            <strong>模型合集</strong>
            <span class="badge" id="modelRouteBadge">0</span>
          </div>
          <div class="model-tools">
            <input id="modelSearchInput" type="text" placeholder="搜索模型" />
          </div>
          <div class="model-list" id="modelList"></div>
        </section>
      </aside>
    </main>

    <footer class="statusbar">
      <span>Settings: {{BASE_URL}}/settings</span>
      <span id="statusText">正在读取配置</span>
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
    let isDirty = false;

    async function loadProviders() {
      setStatus("正在读取配置");
      const response = await apiFetch("/api/providers");
      const data = await response.json();
      const providers = data.providers || [];
      const active = providers.find(p => p.active && p.enabled) || providers[0] || {};
      if (!selectedId) selectedId = active.id || "";
      const selected = providers.find(p => p.id === selectedId) || active || {};
      currentData = data;

      renderProviderList(providers, active, selected);
      renderProvider(selected, data);
      setStatus("配置已同步");
    }

    function renderProviderList(providers, active, selected) {
      document.getElementById("providerCount").textContent = providers.length + " 个";
      document.getElementById("activeBadge").textContent = active.name ? "当前：" + active.name : "未配置";
      document.getElementById("providerList").innerHTML = providers.map(provider => {
        const keyCount = (provider.keys || []).length;
        const readyCount = (provider.keys || []).filter(validKey).length;
        const stateClass = provider.active ? "good" : (readyCount > 0 ? "warn" : "");
        const selectedClass = provider.id === selected.id ? " selected" : "";
        const enabledText = provider.enabled === false ? "已停用" : "已启用";
        return '<button class="provider-item' + selectedClass + '" type="button" onclick="selectProvider(' + "'" + escapeJS(provider.id) + "'" + ')">' +
          '<span class="dot ' + stateClass + '"></span>' +
          '<span>' +
            '<span class="provider-title"><strong>' + escapeHtml(provider.name || provider.id || "未命名") + '</strong><span class="badge">' + enabledText + '</span></span>' +
            '<span class="provider-meta">' +
              '<span>' + keyCount + ' keys · ' + readyCount + ' 可用</span>' +
              '<span>' + escapeHtml(provider.default_model || provider.base_url || "-") + '</span>' +
            '</span>' +
          '</span>' +
        '</button>';
      }).join("") || '<div class="empty">暂无中转站</div>';
    }

    function renderProvider(provider, data) {
      provider = normalizeProvider(provider || {});
      currentProvider = JSON.parse(JSON.stringify(provider));
      isDirty = false;

      const active = provider.active && provider.enabled !== false;
      document.getElementById("providerName").textContent = provider.name || "未命名中转站";
      document.getElementById("endpointState").textContent = active ? "当前使用" : (provider.enabled === false ? "已停用" : "未激活");
      document.getElementById("endpointState").className = "badge " + (active ? "green" : "warn");
      document.getElementById("endpointBadge").textContent = provider.enabled === false ? "Disabled" : (provider.active ? "Active" : "Ready");
      document.getElementById("endpointBadge").className = "badge " + (provider.active ? "green" : "");
      document.getElementById("endpointSummary").textContent = provider.base_url || "-";

      document.getElementById("nameInput").value = provider.name || "";
      document.getElementById("idInput").value = provider.id || "";
      document.getElementById("baseInput").value = provider.base_url || "";
      document.getElementById("defaultModelInput").value = provider.default_model || "";
      document.getElementById("sortInput").value = provider.sort || 10;
      document.getElementById("enabledInput").checked = provider.enabled !== false;
      document.getElementById("proxyInput").value = data.proxy_url || "";

      renderStatus(data, provider);
      renderKeyRoutes(provider);
      renderModels(data, provider);
    }

    function renderStatus(data, provider) {
      const appServer = data.app_server || {};
      const appServerText = appServer.enabled ? ((appServer.status || "pending") + (appServer.url ? " · " + appServer.url : "")) : "disabled";
      document.getElementById("proxyMetric").textContent = data.proxy_url || "-";
      document.getElementById("appServerMetric").textContent = appServerText;
      document.getElementById("appServerTokenMetric").textContent = appServer.token_file || "-";
      document.getElementById("providerMetric").textContent = provider.name || "-";
      document.getElementById("defaultModelMetric").textContent = actualDefaultModel(provider, data);
      document.getElementById("modelMetric").textContent = (data.models || []).length + " 个模型";
    }

    function renderKeyRoutes(provider) {
      const keys = provider.keys || [];
      document.getElementById("keyRoutes").innerHTML = keys.map((key, index) => {
        const ready = validKey(key);
        return '<article class="key-row" data-key-index="' + index + '">' +
          '<div class="field">' +
            '<label>Key 名称</label>' +
            '<input class="key-name" type="text" value="' + escapeAttr(key.name || "") + '" placeholder="例如 GPT 主 Key">' +
            '<div class="compact-controls">' +
              '<label class="toggle-row"><input class="key-enabled" type="checkbox" ' + (key.enabled !== false ? "checked" : "") + '> 启用</label>' +
              '<label class="toggle-row"><input class="key-default" type="radio" name="defaultKey" ' + (key.default ? "checked" : "") + ' onchange="makeDefaultKey(' + index + ')"> 默认</label>' +
            '</div>' +
          '</div>' +
          '<div class="field">' +
            '<label>API Key</label>' +
            '<span class="secret-field">' +
              '<input class="key-api" type="password" value="' + escapeAttr(key.api_key || "") + '" placeholder="sk-...">' +
              '<button class="btn icon" type="button" onclick="toggleKeyVisibility(this)" aria-label="显示或隐藏 API Key" title="显示或隐藏 API Key">' + eyeIcon(false) + '</button>' +
            '</span>' +
          '</div>' +
          '<div class="field">' +
            '<label>手动模型</label>' +
            '<input class="key-models" type="text" value="' + escapeAttr(key.models || "") + '" placeholder="留空时自动读取 /v1/models">' +
          '</div>' +
          '<div class="key-tools">' +
            '<button class="btn icon ghost" type="button" onclick="moveKey(' + index + ', -1)" title="上移">↑</button>' +
            '<button class="btn icon ghost" type="button" onclick="moveKey(' + index + ', 1)" title="下移">↓</button>' +
            '<button class="btn icon ghost" type="button" title="' + (ready ? "Key 已配置" : "Key 未配置") + '">' + (ready ? "✓" : "!") + '</button>' +
            '<button class="btn icon danger" type="button" onclick="removeKey(' + index + ')" title="删除">×</button>' +
          '</div>' +
        '</article>';
      }).join("") || '<div class="empty">暂无 Key</div>';
    }

    function renderModels(data, provider) {
      const query = (document.getElementById("modelSearchInput").value || "").toLowerCase();
      const routes = data.routes || {};
      const defaultModel = actualDefaultModel(provider, data);
      const models = (data.models || []).filter(model => model.toLowerCase().includes(query));
      document.getElementById("modelRouteBadge").textContent = models.length + " 个";
      document.getElementById("modelList").innerHTML = models.slice(0, 80).map(model => {
        const route = routes[model] || {};
        const routeProvider = ((currentData.providers || []).find(item => item.id === route.provider_id)) || provider;
        const keyName = keyDisplayName(routeProvider, route.key_id);
        const source = (model === defaultModel ? "默认 · " : "") + (route.manual ? "手动" : "自动");
        return '<div class="model-item">' +
          '<strong>' + escapeHtml(model) + '</strong>' +
          '<span>' + escapeHtml(keyName || "-") + ' · ' + source + '</span>' +
        '</div>';
      }).join("") || '<div class="empty">暂无模型</div>';
    }

    function selectProvider(id) {
      selectedId = id;
      loadProviders();
    }

    function normalizeProvider(provider) {
      provider.id = provider.id || "";
      provider.name = provider.name || "";
      provider.enabled = provider.enabled !== false;
      provider.sort = provider.sort || 10;
      provider.keys = provider.keys || [];
      if (provider.keys.length === 0) {
        provider.keys.push(emptyKey(1, true));
      }
      return provider;
    }

    function emptyKey(index, makeDefault) {
      return {
        id: "key-" + index,
        name: "",
        api_key: "",
        enabled: true,
        sort: index * 10,
        default: !!makeDefault,
        models: ""
      };
    }

    function collectProvider() {
      const provider = JSON.parse(JSON.stringify(currentProvider || {}));
      provider.name = document.getElementById("nameInput").value.trim();
      provider.id = document.getElementById("idInput").value.trim() || provider.id || idFromName(provider.name);
      provider.base_url = document.getElementById("baseInput").value.trim();
      provider.default_model = document.getElementById("defaultModelInput").value.trim();
      provider.sort = Number(document.getElementById("sortInput").value || 10);
      provider.enabled = document.getElementById("enabledInput").checked;
      provider.keys = Array.from(document.querySelectorAll("[data-key-index]")).map((row, index) => {
        const old = (currentProvider.keys || [])[Number(row.dataset.keyIndex)] || {};
        return {
          id: old.id || ("key-" + (index + 1)),
          name: row.querySelector(".key-name").value.trim(),
          api_key: row.querySelector(".key-api").value.trim(),
          enabled: row.querySelector(".key-enabled").checked,
          sort: (index + 1) * 10,
          default: row.querySelector(".key-default").checked,
          models: row.querySelector(".key-models").value.trim()
        };
      });
      if (!provider.keys.some(key => key.default) && provider.keys.length > 0) {
        provider.keys[0].default = true;
      }
      return provider;
    }

    function idFromName(name) {
      const clean = String(name || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
      return clean || "provider-" + (((currentData.providers || []).length || 0) + 1);
    }

    async function saveProvider() {
      const provider = collectProvider();
      const button = document.getElementById("saveButton");
      button.disabled = true;
      button.textContent = "保存中";
      setStatus("正在保存");
      try {
        const response = await apiFetch("/api/providers", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(provider)
        });
        if (!response.ok) throw new Error(await response.text());
        selectedId = provider.id;
        await loadProviders();
        return true;
      } catch (error) {
        alert("保存失败：" + error.message);
        setStatus("保存失败");
        return false;
      } finally {
        button.disabled = false;
        button.textContent = "保存";
      }
    }

    async function activateProvider() {
      if (!(await saveProvider())) return;
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

    async function refreshModels() {
      const button = document.getElementById("refreshButton");
      button.disabled = true;
      button.textContent = "刷新中";
      setStatus("正在刷新模型");
      try {
        const response = await apiFetch("/api/models/refresh", { method: "POST" });
        if (!response.ok) throw new Error(await response.text());
        await loadProviders();
      } catch (error) {
        alert("刷新失败：" + error.message);
        setStatus("刷新失败");
      } finally {
        button.disabled = false;
        button.textContent = "刷新模型";
      }
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
        keys: [emptyKey(1, true)]
      };
      currentData.providers = currentData.providers || [];
      renderProvider(provider, currentData);
      setDirty();
    }

    function addKey() {
      currentProvider = collectProvider();
      currentProvider.keys = currentProvider.keys || [];
      currentProvider.keys.push(emptyKey(currentProvider.keys.length + 1, currentProvider.keys.length === 0));
      renderProvider(currentProvider, currentData);
      setDirty();
    }

    function removeKey(index) {
      currentProvider = collectProvider();
      if ((currentProvider.keys || []).length <= 1) {
        alert("至少保留一个 Key");
        return;
      }
      currentProvider.keys.splice(index, 1);
      if (!currentProvider.keys.some(key => key.default)) {
        currentProvider.keys[0].default = true;
      }
      renderProvider(currentProvider, currentData);
      setDirty();
    }

    function moveKey(index, direction) {
      currentProvider = collectProvider();
      const next = index + direction;
      if (next < 0 || next >= currentProvider.keys.length) return;
      const keys = currentProvider.keys;
      const item = keys[index];
      keys[index] = keys[next];
      keys[next] = item;
      renderProvider(currentProvider, currentData);
      setDirty();
    }

    function makeDefaultKey(index) {
      currentProvider = collectProvider();
      currentProvider.keys = (currentProvider.keys || []).map((key, keyIndex) => {
        key.default = keyIndex === index;
        return key;
      });
      renderProvider(currentProvider, currentData);
      setDirty();
    }

    function toggleKeyVisibility(button) {
      const input = button.parentElement.querySelector(".key-api");
      const visible = input.type === "text";
      input.type = visible ? "password" : "text";
      button.innerHTML = eyeIcon(!visible);
    }

    function validKey(key) {
      return key && key.enabled !== false && key.api_key && key.api_key.toLowerCase() !== "sk-xx";
    }

    function keyDisplayName(provider, keyId) {
      const key = ((provider && provider.keys) || []).find(item => item.id === keyId);
      return (key && (key.name || key.id)) || keyId || "key";
    }

    function actualDefaultModel(provider, data) {
      return (provider && provider.default_model) || ((data && data.models) || [])[0] || "-";
    }

    function eyeIcon(visible) {
      if (visible) {
        return '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M3 3l18 18"></path><path d="M10.6 10.6a2 2 0 0 0 2.8 2.8"></path><path d="M9.9 4.2A10.8 10.8 0 0 1 12 4c5 0 8.5 3.5 10 8a13.5 13.5 0 0 1-3.1 4.8"></path><path d="M6.6 6.6A13.3 13.3 0 0 0 2 12c1.5 4.5 5 8 10 8 1.4 0 2.7-.3 3.9-.8"></path></svg>';
      }
      return '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"></path><circle cx="12" cy="12" r="3"></circle></svg>';
    }

    function setDirty() {
      isDirty = true;
      document.getElementById("statusText").textContent = "有未保存更改";
    }

    function setStatus(text) {
      if (!isDirty) {
        document.getElementById("statusText").textContent = text;
      }
    }

    function escapeHtml(value) {
      return String(value || "").replace(/[&<>"']/g, ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]));
    }

    function escapeAttr(value) {
      return escapeHtml(value);
    }

    function escapeJS(value) {
      return String(value || "").replace(/\\/g, "\\\\").replace(/'/g, "\\'").replace(/\r/g, "").replace(/\n/g, "");
    }

    document.getElementById("refreshButton").addEventListener("click", refreshModels);
    document.getElementById("saveButton").addEventListener("click", saveProvider);
    document.getElementById("activateButton").addEventListener("click", activateProvider);
    document.getElementById("addKeyButton").addEventListener("click", addKey);
    document.getElementById("addProviderButton").addEventListener("click", addProvider);
    document.getElementById("modelSearchInput").addEventListener("input", () => renderModels(currentData, currentProvider));
    document.querySelector(".workspace").addEventListener("input", setDirty);
    loadProviders().catch(error => {
      alert("读取失败：" + error.message);
      setStatus("读取失败");
    });
  </script>
</body>
</html>`
