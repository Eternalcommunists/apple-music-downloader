package app

// The GUI assets are embedded as Go string constants rather than files on disk
// so the binary stays a single self-contained executable.

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Apple Music 下载器</title>
<link rel="stylesheet" href="/app.css?v=4">
</head>
<body>

<!-- Shown only when the local server stops answering, so a dropped connection
     is explained instead of surfacing as a bare "Failed to fetch". -->
<div class="conn-banner hidden" id="connBanner">
  <div class="conn-text">
    <strong>与本地服务断开连接</strong>
    <span>程序可能已关闭或重启。界面会自动重连；若长时间无法恢复，请重新启动程序。</span>
  </div>
  <div class="conn-actions">
    <span class="conn-state" id="connState">重连中…</span>
    <button class="btn ghost small" id="connReloadBtn">刷新页面</button>
  </div>
</div>

<div class="shell">

  <aside class="sidebar">
    <div class="side-brand">
      <span class="brand-mark">&#9835;</span>
      <div class="side-brand-text">
        <strong>Apple Music</strong>
        <span>下载器</span>
      </div>
    </div>

    <nav class="side-nav" id="sideNav">
      <button class="nav-item active" data-view="download">
        <span class="nav-icon">&#8681;</span><span>下载</span>
      </button>
      <button class="nav-item" data-view="tasks">
        <span class="nav-icon">&#9201;</span><span>任务</span>
        <span class="nav-badge hidden" id="taskBadge">0</span>
      </button>
      <button class="nav-item" data-view="history">
        <span class="nav-icon">&#9776;</span><span>历史</span>
      </button>
      <button class="nav-item" data-view="settings">
        <span class="nav-icon">&#9881;</span><span>设置</span>
      </button>
      <button class="nav-item" data-view="about">
        <span class="nav-icon">&#9432;</span><span>关于</span>
      </button>
    </nav>

    <div class="side-foot">
      <div class="status" id="serviceStatus">
        <span class="dot" id="serviceDot"></span>
        <span id="serviceText">检查服务…</span>
      </div>
      <button class="btn ghost small hidden" id="loginBtn">登录</button>
      <div class="side-version" id="sideVersion" title="点击查看关于">v—</div>
    </div>
  </aside>

  <div class="content">
    <main>

  <section class="view" id="view-download">

  <section class="card" id="inputCard">
    <label class="field-label" for="urlInput">专辑 / 单曲链接</label>
    <div class="row">
      <input id="urlInput" type="url" spellcheck="false" autocomplete="off"
             placeholder="https://music.apple.com/cn/album/... 或 .../song/...">
      <button class="btn primary" id="probeBtn">获取信息</button>
    </div>
    <p class="hint">支持专辑、单曲、播放列表、电台、音乐视频、歌手页面。带 <code>?i=</code> 的专辑链接只下载该单曲。</p>
    <div class="banner error hidden" id="probeError"></div>
  </section>

  <section class="card" id="searchCard">
    <label class="field-label" for="searchInput">搜索音乐 / 专辑</label>
    <div class="row">
      <input id="searchInput" type="search" spellcheck="false" autocomplete="off"
             placeholder="输入歌名、专辑名或歌手名…">
      <button class="btn primary" id="searchBtn">搜索</button>
    </div>
    <div class="tabs" id="searchTabs">
      <button class="tab active" data-type="song">单曲</button>
      <button class="tab" data-type="album">专辑</button>
      <button class="tab" data-type="artist">歌手</button>
    </div>
    <div class="banner error hidden" id="searchError"></div>
    <ul class="results hidden" id="searchResults"></ul>
    <div class="pager hidden" id="searchPager">
      <button class="btn ghost small" id="prevPageBtn">上一页</button>
      <span class="muted" id="pageLabel">—</span>
      <button class="btn ghost small" id="nextPageBtn">下一页</button>
    </div>
  </section>

  <section class="card hidden" id="infoCard">
    <div class="media">
      <img id="artwork" class="artwork" alt="封面" />
      <div class="media-meta">
        <span class="kind-badge" id="kindBadge">专辑</span>
        <h2 id="mediaName">—</h2>
        <p class="artist" id="mediaArtist">—</p>
        <div class="facts" id="mediaFacts"></div>
      </div>
    </div>

    <div class="divider"></div>

    <h3 class="section-title">音质</h3>
    <div class="qualities" id="qualityList"></div>

    <div class="tracks-head">
      <h3 class="section-title">曲目 <span class="muted" id="trackCountLabel"></span></h3>
      <button class="btn ghost small" id="toggleTracks">展开</button>
    </div>
    <ol class="tracks hidden" id="trackList"></ol>

    <div class="divider"></div>

    <div class="save-to">
      <span class="muted">保存位置</span>
      <code id="savePath">—</code>
    </div>

    <div class="actions">
      <button class="btn ghost" id="cancelBtn">取消</button>
      <button class="btn primary" id="downloadBtn">确认下载</button>
    </div>
    <div class="banner error hidden" id="downloadError"></div>
  </section>

  <section class="card hidden" id="progressCard">
    <div class="progress-head">
      <div>
        <span class="kind-badge" id="pKind">—</span>
        <h2 id="pName">—</h2>
        <p class="artist" id="pArtist">—</p>
      </div>
      <div class="quality-tag" id="pQuality">—</div>
    </div>

    <div class="bar-wrap">
      <div class="bar" id="progressBar"></div>
    </div>
    <div class="progress-stats">
      <span id="pPercent">0%</span>
      <span id="pCounts">—</span>
    </div>
    <p class="current" id="pCurrent">准备中…</p>

    <details class="log-wrap">
      <summary>下载日志</summary>
      <pre class="log" id="logPane"></pre>
    </details>
  </section>

  <section class="card done hidden" id="doneCard">
    <div class="done-icon" id="doneIcon">&#10003;</div>
    <h2 id="doneTitle">已下载完成</h2>
    <p class="muted" id="doneSub">—</p>
    <p class="muted done-quality hidden" id="doneQuality">—</p>
    <div class="done-files" id="doneFiles"></div>
    <div class="actions center">
      <button class="btn primary" id="revealBtn">打开文件位置</button>
      <button class="btn ghost" id="againBtn">继续下载</button>
    </div>
    <div class="banner error hidden" id="revealError"></div>
  </section>

  </section><!-- /view-download -->

  <section class="view hidden" id="view-tasks">
    <section class="card">
      <h2 class="view-title">任务</h2>
      <p class="muted small-note">正在进行的下载。进度每秒刷新。</p>
      <div id="taskList" class="job-list">
        <p class="muted">暂无进行中的任务。</p>
      </div>
    </section>
  </section>

  <section class="view hidden" id="view-history">
    <section class="card">
      <div class="view-head">
        <div>
          <h2 class="view-title">历史</h2>
          <p class="muted small-note" id="historyCount">—</p>
        </div>
        <button class="btn ghost small" id="clearHistoryBtn">清空历史</button>
      </div>
      <div id="historyList" class="job-list">
        <p class="muted">还没有下载记录。</p>
      </div>
    </section>
  </section>

  <section class="view hidden" id="view-settings">
    <section class="card">
      <h2 class="view-title">设置</h2>
      <p class="muted small-note">
        配置文件：<code id="cfgPathDisplay">config.yaml</code>
        （与 amdl.exe 同目录）。修改后重启程序生效。
      </p>

      <div class="divider"></div>
      <h3 class="section-title">连接</h3>
      <dl class="kv">
        <dt>lite-server</dt><dd><code id="setLite">—</code></dd>
        <dt>登录状态</dt><dd id="setLogin">—</dd>
        <dt>可用区域</dt><dd id="setRegions">—</dd>
      </dl>

      <div class="divider"></div>
      <h3 class="section-title">保存位置</h3>
      <dl class="kv">
        <dt>无损 (ALAC)</dt><dd><code id="setAlac">—</code></dd>
        <dt>杜比全景声</dt><dd><code id="setAtmos">—</code></dd>
        <dt>高品质 (AAC)</dt><dd><code id="setAac">—</code></dd>
        <dt>音乐视频</dt><dd><code id="setMv">—</code></dd>
      </dl>

      <div class="divider"></div>
      <h3 class="section-title">音质</h3>
      <dl class="kv">
        <dt>ALAC 采样率上限</dt><dd id="setAlacMax">—</dd>
        <dt>Atmos 比特率上限</dt><dd id="setAtmosMax">—</dd>
        <dt>AAC 类型</dt><dd id="setAacType">—</dd>
        <dt>仅无损</dt><dd id="setLosslessOnly">—</dd>
        <dt>ALAC 修复 / 校验</dt><dd id="setAlacFix">—</dd>
      </dl>

      <div class="divider"></div>
      <h3 class="section-title">本地数据</h3>
      <p class="muted small-note">
        只保存在本机，不含账号密码。删除文件即可重置对应状态。
      </p>
      <dl class="kv">
        <dt>下载历史</dt><dd><code id="setHistoryFile">—</code></dd>
        <dt>窗口状态</dt><dd><code id="setStateFile">—</code></dd>
      </dl>

      <div class="actions">
        <button class="btn ghost small" id="reloadSettingsBtn">刷新</button>
      </div>
    </section>
  </section>

  <section class="view hidden" id="view-about">
    <section class="card">
      <div class="about-head">
        <span class="brand-mark big">&#9835;</span>
        <div>
          <h2 class="view-title">Apple Music 下载器</h2>
          <p class="muted" id="aboutTagline">ALAC / 杜比全景声 / AAC 下载工具</p>
        </div>
      </div>

      <div class="divider"></div>
      <dl class="kv">
        <dt>版本</dt><dd id="aboutVersion">—</dd>
        <dt>提交</dt><dd><code id="aboutCommit">—</code></dd>
        <dt>构建时间</dt><dd id="aboutBuild">—</dd>
        <dt>运行时</dt><dd id="aboutGo">—</dd>
      </dl>

      <div class="divider"></div>
      <p class="modal-note">
        本工具仅用于下载你有权访问的内容，请遵守 Apple 服务条款与当地法律。
        下载的音乐仅供个人使用。
      </p>
      <p class="modal-note">
        需要 wrapper-lite 作为后端解密服务，并使用你自己的 Apple Music 账号登录。
        凭据与令牌只保存在本机，不会上传。
      </p>
    </section>
  </section>

    </main>
  </div>
</div>

<!-- Sign-in dialog. Shown only when wrapper-lite has no usable session, so a
     normal run goes straight to the download UI without asking again. -->
<div class="modal-backdrop hidden" id="loginModal">
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="loginTitle">
    <h2 id="loginTitle">登录 Apple Music</h2>
    <p class="modal-sub" id="loginSub">
      首次使用需要登录一次，之后会在本地缓存令牌，无需重复验证。
    </p>

    <label class="field-label" for="loginUser">Apple ID</label>
    <input id="loginUser" type="text" autocomplete="username" spellcheck="false"
           placeholder="你的 Apple ID">

    <label class="field-label" for="loginPass">密码</label>
    <input id="loginPass" type="password" autocomplete="current-password"
           placeholder="Apple ID 密码">

    <label class="field-label" for="loginCode">双重验证码（可选）</label>
    <input id="loginCode" type="text" inputmode="numeric" maxlength="6"
           placeholder="6 位验证码，没开启两步验证可留空">

    <div class="banner error hidden" id="loginError"></div>

    <details class="log-wrap hidden" id="loginLogWrap">
      <summary>登录输出</summary>
      <pre class="log" id="loginLog"></pre>
    </details>

    <div class="actions">
      <button class="btn ghost" id="loginLaterBtn">稍后再说</button>
      <button class="btn primary" id="loginSubmitBtn">登录</button>
    </div>

    <p class="modal-note">
      凭据仅用于调用本机的 wrapper-lite 完成 Apple 验证，不会保存到磁盘。
      登录命令由 <code>config.yaml</code> 的 <code>general.lite-login-command</code> 指定。
    </p>
  </div>
</div>

<!-- In-page preview player. Apple publishes a single AAC preview clip per track,
     so this plays that clip; the quality selector governs the download only. -->
<div class="player hidden" id="player">
  <button class="player-btn" id="playerToggle" aria-label="播放/暂停">
    <span id="playerIcon">&#9654;</span>
  </button>

  <div class="player-body">
    <div class="player-title" id="playerTitle">—</div>
    <div class="player-meta">
      <span class="player-tag">试听 · AAC 预览</span>
      <span class="muted" id="playerNote">音质选择仅影响下载文件</span>
    </div>
  </div>

  <div class="player-seek">
    <span class="player-time" id="playerNow">0:00</span>
    <input type="range" id="playerRange" min="0" max="1000" value="0" step="1"
           aria-label="播放进度">
    <span class="player-time" id="playerDur">0:00</span>
  </div>

  <button class="player-btn ghost" id="playerPrev" aria-label="上一首">&#9198;</button>
  <button class="player-btn ghost" id="playerNext" aria-label="下一首">&#9197;</button>
  <button class="player-btn ghost" id="playerClose" aria-label="关闭播放器">&times;</button>
</div>

<script>window.AMDL_TOKEN = "__TOKEN__";</script>
<script src="/app.js?v=3"></script>
</body>
</html>`

const appCSS = `:root {
  --bg: #0e1014;
  --surface: #171a21;
  --surface-2: #1f232c;
  --border: #2a2f3a;
  --text: #e8eaf0;
  --muted: #9aa2b1;
  --accent: #fa2d48;
  --accent-soft: rgba(250, 45, 72, 0.12);
  --ok: #31c48d;
  --warn: #f0a92c;
  --radius: 14px;
}

* { box-sizing: border-box; }

html, body {
  margin: 0;
  padding: 0;
  background: var(--bg);
  color: var(--text);
  font-family: "Segoe UI", "Microsoft YaHei", system-ui, -apple-system, sans-serif;
  font-size: 15px;
  line-height: 1.5;
}

body {
  min-height: 100vh;
  background:
    radial-gradient(1000px 500px at 15% -10%, rgba(250, 45, 72, 0.10), transparent 60%),
    radial-gradient(800px 400px at 90% 0%, rgba(90, 130, 255, 0.08), transparent 55%),
    var(--bg);
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 22px 32px;
  border-bottom: 1px solid var(--border);
  backdrop-filter: blur(8px);
}

/* ---------- app shell: sidebar + content ---------- */

.conn-banner {
  position: sticky;
  top: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 11px 20px;
  background: rgba(250, 45, 72, 0.14);
  border-bottom: 1px solid rgba(250, 45, 72, 0.4);
  backdrop-filter: blur(10px);
}
.conn-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.conn-text strong { font-size: 13.5px; color: #ff9aab; }
.conn-text span { font-size: 12px; color: var(--muted); }
.conn-actions { display: flex; align-items: center; gap: 12px; flex-shrink: 0; }
.conn-state { font-size: 12px; color: var(--muted); }

.shell { display: flex; min-height: 100vh; }

.sidebar {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: 216px;
  display: flex;
  flex-direction: column;
  padding: 18px 14px 14px;
  border-right: 1px solid var(--border);
  background: rgba(18, 21, 27, 0.86);
  backdrop-filter: blur(12px);
  z-index: 30;
}

.content {
  flex: 1;
  min-width: 0;
  margin-left: 216px;
}

.side-brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 4px 8px 20px;
}
.side-brand .brand-mark { width: 38px; height: 38px; font-size: 19px; border-radius: 11px; }
.side-brand-text { display: flex; flex-direction: column; line-height: 1.25; min-width: 0; }
.side-brand-text strong { font-size: 14px; font-weight: 650; }
.side-brand-text span { font-size: 12px; color: var(--muted); }

.side-nav { display: flex; flex-direction: column; gap: 3px; flex: 1; }

.nav-item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--muted);
  font-size: 13.5px;
  font-weight: 600;
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition: background 120ms, color 120ms;
}
.nav-item:hover { background: var(--surface-2); color: var(--text); }
.nav-item.active {
  background: var(--accent-soft);
  color: #ff8a9c;
  box-shadow: inset 2px 0 0 var(--accent);
}
.nav-icon { width: 18px; text-align: center; font-size: 14px; }

.nav-badge {
  margin-left: auto;
  min-width: 20px;
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  text-align: center;
}

.side-foot {
  display: flex;
  flex-direction: column;
  gap: 9px;
  padding: 12px 8px 4px;
  border-top: 1px solid var(--border);
}
.side-foot .status { font-size: 12px; }
.side-foot .btn { width: 100%; justify-content: center; }

.side-version {
  color: var(--muted);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
  cursor: pointer;
  padding-top: 2px;
  opacity: 0.75;
  transition: opacity 120ms, color 120ms;
}
.side-version:hover { opacity: 1; color: var(--text); }

.view-title { margin: 0 0 6px; font-size: 19px; font-weight: 650; }
.small-note { font-size: 12.5px; margin: 0 0 16px; }

.view-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.kv {
  display: grid;
  grid-template-columns: minmax(120px, 190px) 1fr;
  gap: 9px 18px;
  margin: 0;
  font-size: 13px;
}
.kv dt { color: var(--muted); }
.kv dd { margin: 0; word-break: break-all; }
.kv code {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 2px 7px;
  font-size: 12.5px;
}

.job-list { display: flex; flex-direction: column; gap: 10px; }

.job {
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--surface-2);
  padding: 13px 15px;
}

.job-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}
.job-name { font-weight: 620; font-size: 14px; word-break: break-word; }
.job-meta { color: var(--muted); font-size: 12.5px; margin-top: 3px; }

.job-status {
  flex-shrink: 0;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 700;
  white-space: nowrap;
  border: 1px solid var(--border);
}
.job-status.running { color: #ffd479; border-color: rgba(240, 169, 44, 0.4); background: rgba(240, 169, 44, 0.1); }
.job-status.completed { color: #6fe0b4; border-color: rgba(49, 196, 141, 0.4); background: rgba(49, 196, 141, 0.1); }
.job-status.failed { color: #ff9aab; border-color: rgba(250, 45, 72, 0.4); background: var(--accent-soft); }

.job-bar {
  height: 5px;
  margin-top: 11px;
  border-radius: 999px;
  background: #0e1117;
  border: 1px solid var(--border);
  overflow: hidden;
}
.job-bar > i {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, var(--accent), #ff8a9c);
  transition: width 320ms ease-out;
}

.job-actions { display: flex; gap: 8px; margin-top: 11px; }

.about-head { display: flex; align-items: center; gap: 15px; }
.brand-mark.big { width: 54px; height: 54px; font-size: 26px; border-radius: 15px; }

/* Collapse to an icon rail when space is tight. The threshold is deliberately
   small so a normal window keeps the labels; only genuinely narrow viewports
   (embedded previews, small tiled windows) fall back to icons. */
@media (max-width: 560px) {
  .sidebar { width: 62px; padding: 14px 8px; }
  .side-brand-text,
  .nav-item > span:not(.nav-icon):not(.nav-badge),
  .side-version { display: none; }
  .nav-item { justify-content: center; }
  .nav-badge {
    position: absolute;
    top: 4px;
    right: 4px;
    margin-left: 0;
  }
  .nav-item { position: relative; }
  .content { margin-left: 62px; }
  .kv { grid-template-columns: 1fr; gap: 3px 0; }
  .kv dt { margin-top: 7px; }
}

@media (max-width: 820px) {
  .kv { grid-template-columns: minmax(110px, 150px) 1fr; }
}

.brand { display: flex; align-items: center; gap: 14px; }

.brand-mark {
  display: grid;
  place-items: center;
  width: 44px; height: 44px;
  border-radius: 12px;
  background: linear-gradient(140deg, var(--accent), #ff6b81);
  color: #fff;
  font-size: 22px;
  box-shadow: 0 8px 24px rgba(250, 45, 72, 0.35);
}

.topbar h1 { margin: 0; font-size: 19px; font-weight: 650; letter-spacing: 0.2px; }
.sub { margin: 2px 0 0; color: var(--muted); font-size: 13px; }

.status { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--muted); }
.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--muted); }
.dot.ok { background: var(--ok); box-shadow: 0 0 0 4px rgba(49, 196, 141, 0.15); }
.dot.bad { background: var(--accent); box-shadow: 0 0 0 4px var(--accent-soft); }

main {
  max-width: 880px;
  margin: 0 auto;
  padding: 28px 24px 60px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* A view is one sidebar destination; only the active one is rendered. */
.view { display: flex; flex-direction: column; gap: 20px; }

.card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 24px;
  box-shadow: 0 18px 40px -28px rgba(0, 0, 0, 0.9);
  animation: rise 220ms ease-out;
}

@keyframes rise {
  from { opacity: 0; transform: translateY(6px); }
  to   { opacity: 1; transform: none; }
}

.hidden { display: none !important; }

.field-label {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--muted);
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.6px;
}

.row { display: flex; gap: 10px; }

input[type="url"] {
  flex: 1;
  min-width: 0;
  padding: 13px 14px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  font-size: 14px;
  font-family: inherit;
  transition: border-color 120ms, box-shadow 120ms;
}

input[type="url"]:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.btn {
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  padding: 12px 20px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  white-space: nowrap;
  transition: transform 90ms, background 120ms, border-color 120ms, opacity 120ms;
}

.btn:hover:not(:disabled) { background: #262b35; }
.btn:active:not(:disabled) { transform: translateY(1px); }
.btn:disabled { opacity: 0.45; cursor: not-allowed; }

.btn.primary {
  background: linear-gradient(140deg, var(--accent), #ff5c72);
  border-color: transparent;
  color: #fff;
  box-shadow: 0 10px 26px -12px rgba(250, 45, 72, 0.9);
}
.btn.primary:hover:not(:disabled) { background: linear-gradient(140deg, #ff3b55, #ff6b80); }

.btn.ghost { background: transparent; }
.btn.small { padding: 7px 12px; font-size: 13px; }

.hint { margin: 10px 0 0; font-size: 12.5px; color: var(--muted); }
.hint code {
  background: var(--surface-2);
  padding: 1px 5px;
  border-radius: 5px;
  font-size: 12px;
}

.banner {
  margin-top: 14px;
  padding: 11px 14px;
  border-radius: 10px;
  font-size: 13.5px;
}

.banner.error {
  background: var(--accent-soft);
  border: 1px solid rgba(250, 45, 72, 0.35);
  color: #ffb3bf;
}

.media { display: flex; gap: 20px; align-items: flex-start; }

.artwork {
  width: 150px; height: 150px;
  border-radius: 12px;
  object-fit: cover;
  background: var(--surface-2);
  border: 1px solid var(--border);
  flex-shrink: 0;
}

.media-meta { min-width: 0; flex: 1; }

.kind-badge {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--accent-soft);
  border: 1px solid rgba(250, 45, 72, 0.3);
  color: #ff8a9c;
  font-size: 11.5px;
  font-weight: 700;
  letter-spacing: 0.5px;
}

h2 { margin: 10px 0 4px; font-size: 21px; font-weight: 650; word-break: break-word; }
.artist { margin: 0; color: var(--muted); font-size: 14.5px; }

.facts {
  margin-top: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.fact {
  padding: 4px 10px;
  border-radius: 8px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12.5px;
  color: var(--muted);
}

.fact.master { color: var(--warn); border-color: rgba(240, 169, 44, 0.35); }
.fact.explicit { color: var(--accent); border-color: rgba(250, 45, 72, 0.35); }

.divider { height: 1px; background: var(--border); margin: 22px 0; }

.section-title {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 700;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.6px;
}

.qualities { display: flex; flex-direction: column; gap: 10px; }

.quality {
  display: flex;
  align-items: flex-start;
  gap: 13px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  cursor: pointer;
  transition: border-color 120ms, background 120ms;
}

.quality:hover:not(.disabled) { border-color: #3a4150; }

.quality.selected {
  border-color: var(--accent);
  background: var(--accent-soft);
}

.quality.disabled { opacity: 0.45; cursor: not-allowed; }

.quality input { margin-top: 3px; accent-color: var(--accent); }

.quality-body { flex: 1; min-width: 0; }
.quality-name { font-weight: 650; font-size: 14.5px; }
.quality-desc { color: var(--muted); font-size: 13px; margin-top: 2px; }
.quality-reason { color: var(--warn); font-size: 12.5px; margin-top: 3px; }

.tracks-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 22px;
}
.tracks-head .section-title { margin: 0; }

.muted { color: var(--muted); font-weight: 400; text-transform: none; letter-spacing: 0; }

.tracks {
  margin: 14px 0 0;
  padding: 0 0 0 6px;
  list-style: none;
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-2);
}

.tabs { display: flex; gap: 6px; margin-top: 14px; }

.tab {
  padding: 7px 16px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: background 120ms, border-color 120ms, color 120ms;
}

.tab:hover { border-color: #3a4150; }
.tab.active { background: var(--accent-soft); border-color: var(--accent); color: #ff8a9c; }

.results {
  margin: 16px 0 0;
  padding: 0;
  list-style: none;
  max-height: 430px;
  overflow-y: auto;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-2);
}

.result {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 11px 14px;
  border-bottom: 1px solid rgba(42, 47, 58, 0.6);
  cursor: pointer;
  transition: background 120ms;
}
.result:last-child { border-bottom: none; }
.result:hover { background: #262b35; }
.result:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }

.result img {
  width: 52px; height: 52px;
  border-radius: 8px;
  object-fit: cover;
  background: #0f1218;
  flex-shrink: 0;
}

.result-body { flex: 1; min-width: 0; }
.result-name { font-weight: 620; font-size: 14px; word-break: break-word; }
.result-meta { color: var(--muted); font-size: 12.5px; margin-top: 2px; }

.result-go {
  color: var(--muted);
  font-size: 12.5px;
  white-space: nowrap;
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 4px 12px;
}
.result:hover .result-go { color: #ff8a9c; border-color: var(--accent); }

.pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  margin-top: 14px;
}

.tracks li {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 9px 14px;
  border-bottom: 1px solid rgba(42, 47, 58, 0.6);
  font-size: 13.5px;
}
.tracks li:last-child { border-bottom: none; }

.track-num { color: var(--muted); min-width: 26px; font-variant-numeric: tabular-nums; }
.track-name { flex: 1; min-width: 0; word-break: break-word; }
.track-dur { color: var(--muted); font-variant-numeric: tabular-nums; font-size: 12.5px; }
.track-e {
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
  border: 1px solid rgba(250, 45, 72, 0.4);
  border-radius: 4px;
  padding: 0 4px;
}

.save-to {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  margin-bottom: 20px;
}
.save-to code {
  background: var(--surface-2);
  border: 1px solid var(--border);
  padding: 5px 9px;
  border-radius: 7px;
  font-size: 12.5px;
  word-break: break-all;
}

.actions { display: flex; gap: 10px; justify-content: flex-end; }
.actions.center { justify-content: center; }

.progress-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.quality-tag {
  padding: 5px 12px;
  border-radius: 999px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12.5px;
  font-weight: 650;
  white-space: nowrap;
}

.bar-wrap {
  margin: 22px 0 10px;
  height: 8px;
  border-radius: 999px;
  background: var(--surface-2);
  overflow: hidden;
  border: 1px solid var(--border);
}

.bar {
  height: 100%;
  width: 0;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent), #ff8a9c);
  transition: width 320ms cubic-bezier(0.16, 1, 0.3, 1);
}

.progress-stats {
  display: flex;
  justify-content: space-between;
  color: var(--muted);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.current {
  margin: 12px 0 0;
  font-size: 13.5px;
  color: #c3cad6;
  word-break: break-word;
  min-height: 20px;
}

.log-wrap { margin-top: 18px; }

.log-wrap summary {
  cursor: pointer;
  font-size: 13px;
  color: var(--muted);
  padding: 6px 0;
  user-select: none;
}

.log {
  margin: 8px 0 0;
  max-height: 220px;
  overflow: auto;
  padding: 12px 14px;
  border-radius: 10px;
  background: #0b0d11;
  border: 1px solid var(--border);
  font-family: "Cascadia Mono", Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
  color: #93a0b4;
  white-space: pre-wrap;
  word-break: break-all;
}

.done { text-align: center; padding: 34px 24px; }

.done-icon {
  width: 62px; height: 62px;
  margin: 0 auto 16px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  font-size: 30px;
  color: #fff;
  background: linear-gradient(140deg, var(--ok), #5fd6a8);
  box-shadow: 0 14px 34px -14px rgba(49, 196, 141, 0.9);
}

.done.failed .done-icon {
  background: linear-gradient(140deg, var(--accent), #ff6b81);
  box-shadow: 0 14px 34px -14px rgba(250, 45, 72, 0.9);
}

.done h2 { margin: 0 0 6px; font-size: 22px; }

.done-files {
  margin: 20px auto;
  max-width: 560px;
  max-height: 180px;
  overflow-y: auto;
  text-align: left;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--surface-2);
}

.done-files:empty { display: none; }

.done-file {
  padding: 8px 14px;
  border-bottom: 1px solid rgba(42, 47, 58, 0.6);
  font-size: 13px;
  word-break: break-all;
}
.done-file:last-child { border-bottom: none; }
.done-file .fname { font-weight: 600; }
.done-file .fpath { color: var(--muted); font-size: 12px; margin-top: 2px; }

.done .actions { margin-top: 8px; }
.done-quality {
  display: inline-block;
  margin: 2px 0 0;
  padding: 4px 12px;
  border-radius: 999px;
  background: var(--surface-2);
  border: 1px solid var(--border);
  font-size: 12.5px;
  color: var(--text);
}

/* ---------- sign-in dialog ---------- */

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgba(6, 8, 12, 0.72);
  backdrop-filter: blur(6px);
  animation: fade 160ms ease-out;
}

@keyframes fade {
  from { opacity: 0; }
  to   { opacity: 1; }
}

.modal {
  width: 100%;
  max-width: 420px;
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 26px;
  box-shadow: 0 30px 70px -30px rgba(0, 0, 0, 0.95);
  animation: rise 200ms ease-out;
}

.modal h2 { margin: 0 0 6px; font-size: 19px; }
.modal-sub { margin: 0 0 20px; color: var(--muted); font-size: 13px; }
.modal .field-label { margin-top: 14px; }
.modal .field-label:first-of-type { margin-top: 0; }

.modal input[type="text"],
.modal input[type="password"] {
  width: 100%;
  padding: 11px 13px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  color: var(--text);
  font-size: 14px;
  font-family: inherit;
  transition: border-color 120ms, box-shadow 120ms;
}

.modal input[type="text"]:focus,
.modal input[type="password"]:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.modal .actions { margin-top: 22px; }

.modal-note {
  margin: 16px 0 0;
  font-size: 12px;
  color: var(--muted);
  line-height: 1.6;
}

.modal-note code {
  background: var(--surface-2);
  padding: 1px 5px;
  border-radius: 5px;
  font-size: 11.5px;
}

.modal .log-wrap { margin-top: 14px; }
.modal .log { max-height: 130px; }

/* ---------- in-page preview player ---------- */

body.has-player main { padding-bottom: 130px; }

.player {
  position: fixed;
  left: 50%;
  bottom: 18px;
  transform: translateX(-50%);
  z-index: 40;
  width: min(900px, calc(100vw - 36px));
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: rgba(23, 26, 33, 0.94);
  backdrop-filter: blur(14px);
  box-shadow: 0 24px 60px -26px rgba(0, 0, 0, 0.95);
  animation: rise 200ms ease-out;
}

.player-btn {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  flex-shrink: 0;
  border-radius: 50%;
  border: none;
  background: linear-gradient(140deg, var(--accent), #ff5c72);
  color: #fff;
  font-size: 16px;
  cursor: pointer;
  transition: transform 90ms, filter 120ms;
}
.player-btn:hover { filter: brightness(1.08); }
.player-btn:active { transform: translateY(1px); }

.player-btn.ghost {
  width: 34px;
  height: 34px;
  background: transparent;
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: 14px;
}
.player-btn.ghost:hover { color: var(--text); border-color: #3a4150; }

.player-body { min-width: 0; flex: 1; }
.player-title {
  font-size: 14px;
  font-weight: 620;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.player-meta { display: flex; gap: 8px; align-items: center; margin-top: 3px; font-size: 12px; }

.player-tag {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--accent-soft);
  border: 1px solid rgba(250, 45, 72, 0.3);
  color: #ff8a9c;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.player-seek { display: flex; align-items: center; gap: 9px; flex-shrink: 0; }
.player-time {
  color: var(--muted);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
  min-width: 34px;
  text-align: center;
}

.player-seek input[type="range"] {
  width: 210px;
  accent-color: var(--accent);
  cursor: pointer;
}

/* Per-track play button in the track list. */
.track-play {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--muted);
  font-size: 11px;
  cursor: pointer;
  transition: color 120ms, border-color 120ms, background 120ms;
}
.track-play:hover { color: #fff; border-color: var(--accent); background: var(--accent-soft); }
.track-play.playing { color: #fff; border-color: var(--accent); background: var(--accent); }

.tracks li.active { background: rgba(250, 45, 72, 0.07); }
.tracks li.active .track-name { color: #ff9aab; }
`

const appJS = `(function () {
  "use strict";

  var TOKEN = window.AMDL_TOKEN || "";
  var state = { probe: null, quality: null, sessionId: null, poll: null, lastLogCount: 0 };

  var $ = function (id) { return document.getElementById(id); };

  function api(path, opts) {
    opts = opts || {};
    opts.headers = Object.assign(
      { "X-AmDL-Token": TOKEN },
      opts.headers || {}
    );
    if (opts.body) {
      opts.headers["Content-Type"] = "application/json";
    }
    return fetch(path, opts).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) {
          throw new Error(data.error || ("请求失败 (" + res.status + ")"));
        }
        return data;
      });
    }).catch(function (err) {
      // A network-level failure means the local server is not answering: the
      // window was closed, or the app was restarted on a different address.
      // Surface that distinctly so the user is not left with a bare
      // "Failed to fetch" and no idea what to do about it.
      onConnectionLost();
      throw err;
    });
  }

  /* ---------- connection watchdog ---------- */

  var conn = { down: false, timer: null, lastOk: Date.now() };

  function onConnectionLost() {
    if (conn.down) { return; }
    conn.down = true;
    $("connBanner").classList.remove("hidden");
    // Keep probing so the banner clears itself once the server is back on the
    // same address (for example after the app is restarted on the fixed port).
    if (!conn.timer) {
      conn.timer = setInterval(pingService, 2500);
    }
  }

  function onConnectionRestored() {
    if (!conn.down) { return; }
    conn.down = false;
    $("connBanner").classList.add("hidden");
    if (conn.timer) { clearInterval(conn.timer); conn.timer = null; }
    // Re-sync everything that may have changed while the server was away.
    checkService();
    loadJobs();
    if (currentView === "settings") { loadSettingsPage(); }
    if (currentView === "about") { loadAbout(); }
  }

  // pingService is a lightweight liveness probe used by the watchdog.
  function pingService() {
    fetch("/api/ping", { headers: { "X-AmDL-Token": TOKEN } })
      .then(function (r) {
        if (r.ok) { onConnectionRestored(); }
      })
      .catch(function () { /* still down */ });
  }

  function reloadPage() {
    location.reload();
  }

  function show(id) { $(id).classList.remove("hidden"); }
  function hide(id) { $(id).classList.add("hidden"); }
  function text(id, v) { $(id).textContent = v == null ? "" : v; }

  function banner(id, message) {
    var el = $(id);
    if (!message) { el.classList.add("hidden"); el.textContent = ""; return; }
    el.textContent = message;
    el.classList.remove("hidden");
  }

  function fmtDuration(ms) {
    if (!ms || ms <= 0) return "";
    var total = Math.round(ms / 1000);
    var m = Math.floor(total / 60);
    var s = total % 60;
    return m + ":" + (s < 10 ? "0" : "") + s;
  }

  function kindLabel(kind) {
    return ({
      album: "专辑", song: "单曲", playlist: "播放列表",
      station: "电台", mv: "音乐视频", artist: "歌手"
    })[kind] || kind;
  }

  function qualityLabel(id) {
    return ({
      alac: "无损 (ALAC)", aac: "高品质 (AAC)",
      atmos: "杜比全景声", ac3: "杜比音效"
    })[id] || id;
  }

  /* ---------- service status & sign-in ---------- */

  var login = { id: null, poll: null, busy: false };

  // checkService decides whether the sign-in dialog is needed. wrapper-lite
  // reports an empty region list when it holds no usable session, which is the
  // single source of truth for "login required". The service caches its tokens
  // on disk, so a normal run never sees this dialog.
  function checkService() {
    api("/api/login/status").then(function (st) {
      applyLoginState(st);
      if (!st.loggedIn) { openLogin(false); }
    }).catch(function () {
      $("serviceDot").className = "dot bad";
      text("serviceText", "服务未连接");
      $("loginBtn").classList.remove("hidden");
    });
  }

  function applyLoginState(st) {
    if (st.loggedIn) {
      $("serviceDot").className = "dot ok";
      text("serviceText", "已登录 · " + ((st.regions || []).join(", ") || "—"));
      $("loginBtn").classList.add("hidden");
      closeLogin();
    } else {
      $("serviceDot").className = "dot bad";
      text("serviceText", st.liteServer ? "未登录" : "未配置 lite-server");
      $("loginBtn").classList.remove("hidden");
    }
  }

  function openLogin(force) {
    banner("loginError", "");
    $("loginModal").classList.remove("hidden");
    $("loginUser").focus();
  }

  function closeLogin() {
    $("loginModal").classList.add("hidden");
    banner("loginError", "");
  }

  function setLoginBusy(busy) {
    login.busy = busy;
    $("loginSubmitBtn").disabled = busy;
    $("loginSubmitBtn").textContent = busy ? "验证中…" : "登录";
    $("loginUser").disabled = busy;
    $("loginPass").disabled = busy;
    $("loginCode").disabled = busy;
  }

  function submitLogin() {
    if (login.busy) { return; }
    banner("loginError", "");

    var user = $("loginUser").value.trim();
    var pass = $("loginPass").value;
    var code = $("loginCode").value.trim();
    if (!user) { banner("loginError", "请输入 Apple ID"); return; }
    if (!pass) { banner("loginError", "请输入密码"); return; }

    setLoginBusy(true);

    api("/api/login", {
      method: "POST",
      body: JSON.stringify({ user: user, pass: pass, code: code })
    }).then(function (res) {
      login.id = res.id;
      // The credentials are now owned by the backend; clear the password from
      // the page so it does not linger in the DOM.
      $("loginPass").value = "";
      startLoginPoll();
    }).catch(function (err) {
      banner("loginError", err.message);
      setLoginBusy(false);
    });
  }

  function startLoginPoll() {
    stopLoginPoll();
    login.poll = setInterval(pollLogin, 1200);
    pollLogin();
  }

  function stopLoginPoll() {
    if (login.poll) { clearInterval(login.poll); login.poll = null; }
  }

  function pollLogin() {
    if (!login.id) { return; }
    api("/api/login/progress?id=" + encodeURIComponent(login.id)).then(function (s) {
      if (s.output && s.output.length) {
        $("loginLogWrap").classList.remove("hidden");
        $("loginLog").textContent = s.output.join("\n");
      }

      if (s.status === "completed") {
        stopLoginPoll();
        setLoginBusy(false);
        // wrapper-lite has the session cached now, so the dialog stays closed.
        applyLoginState({ loggedIn: true, regions: s.regions, liteServer: "" });
        return;
      }
      if (s.status === "failed") {
        stopLoginPoll();
        setLoginBusy(false);
        banner("loginError", s.errorText || s.detail || s.message || "登录失败");
      }
    }).catch(function () {});
  }

  /* ---------- search ---------- */

  var search = { q: "", type: "song", offset: 0, limit: 15, hasMore: false, busy: false };

  function runSearch(offset) {
    banner("searchError", "");
    var q = $("searchInput").value.trim();
    if (!q) { banner("searchError", "请输入关键词"); return; }
    if (search.busy) { return; }

    search.busy = true;
    search.q = q;
    search.offset = offset || 0;
    var btn = $("searchBtn");
    btn.disabled = true;
    btn.textContent = "搜索中…";

    api("/api/search?q=" + encodeURIComponent(q) +
        "&type=" + encodeURIComponent(search.type) +
        "&offset=" + search.offset)
      .then(function (res) {
        search.limit = res.limit || 15;
        search.hasMore = !!res.hasMore;
        renderResults(res.hits || []);
        renderPager();
      })
      .catch(function (err) {
        banner("searchError", err.message);
        renderResults([]);
        hide("searchPager");
      })
      .finally(function () {
        search.busy = false;
        btn.disabled = false;
        btn.textContent = "搜索";
      });
  }

  function renderResults(hits) {
    var box = $("searchResults");
    box.innerHTML = "";

    if (!hits.length) {
      box.classList.add("hidden");
      return;
    }

    hits.forEach(function (h) {
      var li = document.createElement("li");
      li.className = "result";
      li.tabIndex = 0;
      li.setAttribute("role", "button");

      if (h.artworkUrl) {
        var img = document.createElement("img");
        img.src = h.artworkUrl.replace("{w}x{h}", "120x120");
        img.alt = "";
        img.loading = "lazy";
        li.appendChild(img);
      }

      var body = document.createElement("div");
      body.className = "result-body";

      var name = document.createElement("div");
      name.className = "result-name";
      name.textContent = h.name || "";
      if (h.explicit) {
        var e = document.createElement("span");
        e.className = "track-e";
        e.textContent = "E";
        e.style.marginLeft = "6px";
        name.appendChild(e);
      }
      body.appendChild(name);

      var meta = document.createElement("div");
      meta.className = "result-meta";
      meta.textContent = hitMeta(h);
      body.appendChild(meta);

      li.appendChild(body);

      var go = document.createElement("span");
      go.className = "result-go";
      go.textContent = h.kind === "artist" ? "查看" : "选择";
      li.appendChild(go);

      // Selecting a hit feeds its URL into the normal probe flow, so the user
      // lands on the same confirmation screen as a pasted link.
      var pick = function () { pickResult(h); };
      li.addEventListener("click", pick);
      li.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); pick(); }
      });

      box.appendChild(li);
    });

    box.classList.remove("hidden");
  }

  function hitMeta(h) {
    var parts = [];
    if (h.kind === "song") {
      if (h.artist) parts.push(h.artist);
      if (h.albumName) parts.push(h.albumName);
      if (h.durationMillis) parts.push(fmtDuration(h.durationMillis));
    } else if (h.kind === "album") {
      if (h.artist) parts.push(h.artist);
      if (h.releaseDate) parts.push(h.releaseDate.slice(0, 4));
      if (h.trackCount) parts.push(h.trackCount + " 首");
    } else if (h.kind === "artist") {
      if (h.genres && h.genres.length) parts.push(h.genres.join(" / "));
    }
    return parts.join(" · ");
  }

  function renderPager() {
    var pager = $("searchPager");
    var page = Math.floor(search.offset / search.limit) + 1;
    text("pageLabel", "第 " + page + " 页");
    $("prevPageBtn").disabled = search.offset <= 0;
    $("nextPageBtn").disabled = !search.hasMore;
    pager.classList.remove("hidden");
  }

  function pickResult(h) {
    if (h.kind === "artist") {
      // An artist page expands into many albums, which the probe reports but
      // the download step cannot consume directly; tell the user to pick an album.
      banner("searchError", "歌手页面请展开后选择具体专辑下载：请把该歌手的专辑链接粘贴到上方输入框");
      return;
    }
    $("urlInput").value = h.url || "";
    probe();
  }

  /* ---------- step 1: probe ---------- */

  function probe() {
    var url = $("urlInput").value.trim();
    banner("probeError", "");
    if (!url) { banner("probeError", "请先粘贴链接"); return; }

    var btn = $("probeBtn");
    btn.disabled = true;
    btn.textContent = "获取中…";

    api("/api/probe?url=" + encodeURIComponent(url)).then(function (res) {
      state.probe = res;
      renderInfo(res);
      show("infoCard");
      hide("progressCard");
      hide("doneCard");
    }).catch(function (err) {
      banner("probeError", err.message);
    }).finally(function () {
      btn.disabled = false;
      btn.textContent = "获取信息";
    });
  }

  function renderInfo(res) {
    text("kindBadge", kindLabel(res.kind));
    text("mediaName", res.name || "—");
    text("mediaArtist", res.artist || "—");
    // The confirmation screen replaces the search box so the user is not
    // tempted to start another probe mid-flow.
    hide("searchCard");

    var art = $("artwork");
    if (res.artworkUrl) {
      art.src = res.artworkUrl.replace("{w}x{h}", "300x300");
      art.classList.remove("hidden");
    } else {
      art.removeAttribute("src");
    }

    var facts = [];
    if (res.releaseDate) facts.push({ t: res.releaseDate.slice(0, 4), cls: "" });
    if (res.trackCount) facts.push({ t: res.trackCount + " 首", cls: "" });
    if (res.genres && res.genres.length) facts.push({ t: res.genres[0], cls: "" });
    if (res.isMaster) facts.push({ t: "Apple Digital Master", cls: "master" });
    if (res.hasExplicit) facts.push({ t: "含脏标曲目", cls: "explicit" });
    if (res.recordLabel) facts.push({ t: res.recordLabel, cls: "" });
    var box = $("mediaFacts");
    box.innerHTML = "";
    facts.forEach(function (f) {
      var el = document.createElement("span");
      el.className = "fact " + (f.cls || "");
      el.textContent = f.t;
      box.appendChild(el);
    });

    renderQualities(res);
    renderTracks(res);
    loadSavePath(res);

    // Pre-select the first available quality.
    var first = (res.qualities || []).filter(function (q) { return q.available; })[0];
    if (first) selectQuality(first.id);
  }

  function renderQualities(res) {
    var box = $("qualityList");
    box.innerHTML = "";
    var list = res.qualities || [];
    if (!list.length) {
      box.innerHTML = '<p class="muted">该类型在下载时展开为多个专辑，音质按专辑自动选择。</p>';
      return;
    }
    list.forEach(function (q) {
      var label = document.createElement("label");
      label.className = "quality" + (q.available ? "" : " disabled");
      label.dataset.id = q.id;

      var radio = document.createElement("input");
      radio.type = "radio";
      radio.name = "quality";
      radio.value = q.id;
      radio.disabled = !q.available;
      radio.addEventListener("change", function () { selectQuality(q.id); });

      var body = document.createElement("div");
      body.className = "quality-body";

      var name = document.createElement("div");
      name.className = "quality-name";
      name.textContent = q.name;

      var desc = document.createElement("div");
      desc.className = "quality-desc";
      desc.textContent = q.description || "";

      body.appendChild(name);
      body.appendChild(desc);

      if (!q.available && q.reason) {
        var reason = document.createElement("div");
        reason.className = "quality-reason";
        reason.textContent = q.reason;
        body.appendChild(reason);
      }

      label.appendChild(radio);
      label.appendChild(body);
      box.appendChild(label);
    });
  }

  function selectQuality(id) {
    state.quality = id;
    var nodes = document.querySelectorAll(".quality");
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i];
      var on = n.dataset.id === id;
      n.classList.toggle("selected", on);
      var r = n.querySelector("input[type=radio]");
      if (r && on) r.checked = true;
    }
    updateSavePath();
  }

  function renderTracks(res) {
    var list = res.tracks || [];
    var playable = list.filter(function (t) { return t.previewUrl; }).length;
    text("trackCountLabel", list.length
      ? "（共 " + list.length + " 首" + (playable ? "・" + playable + " 首可试听" : "") + "）"
      : "");

    var box = $("trackList");
    box.innerHTML = "";
    list.forEach(function (t, idx) {
      var li = document.createElement("li");

      var num = document.createElement("span");
      num.className = "track-num";
      num.textContent = (t.disc > 1 ? t.disc + "-" : "") + (t.number || "");
      li.appendChild(num);

      var name = document.createElement("span");
      name.className = "track-name";
      name.textContent = t.name || "";
      li.appendChild(name);

      if (t.explicit) {
        var e = document.createElement("span");
        e.className = "track-e";
        e.textContent = "E";
        li.appendChild(e);
      }

      var dur = document.createElement("span");
      dur.className = "track-dur";
      dur.textContent = fmtDuration(t.durationMillis);
      li.appendChild(dur);

      // Play button. Disabled-looking when Apple has no preview for this track.
      var play = document.createElement("button");
      play.className = "track-play";
      play.type = "button";
      play.textContent = "\u25B6";
      play.title = t.previewUrl ? "试听 30 秒片段" : "该曲目无试听片段";
      play.disabled = !t.previewUrl;
      if (!t.previewUrl) { play.style.opacity = "0.35"; play.style.cursor = "not-allowed"; }
      play.addEventListener("click", function (ev) {
        ev.stopPropagation();
        if (!t.previewUrl) { return; }
        // Clicking the playing row again pauses it, like a toggle.
        if (player.index === idx && player.audio && !player.audio.paused) {
          player.audio.pause();
          return;
        }
        playTrack(list, idx);
      });
      li.appendChild(play);

      box.appendChild(li);
    });

    $("toggleTracks").disabled = list.length === 0;
  }

  var settings = null;

  function loadSavePath(res) {
    if (settings) { updateSavePath(); return; }
    api("/api/settings").then(function (s) {
      settings = s;
      updateSavePath();
    }).catch(function () {
      text("savePath", "—");
    });
  }

  // updateSavePath reflects the folder the selected quality writes to. A music
  // video always lands in the MV folder regardless of audio quality.
  function updateSavePath() {
    if (!settings || !state.probe) { return; }
    var dir = settings.alac;
    if (state.probe.kind === "mv") dir = settings.mv;
    else if (state.quality === "atmos") dir = settings.atmos;
    else if (state.quality === "aac") dir = settings.aac;
    text("savePath", dir || "—");
  }

  /* ---------- step 2: download ---------- */

  function startDownload() {
    banner("downloadError", "");
    if (!state.probe) return;
    if (!state.quality) { banner("downloadError", "请选择音质"); return; }
    if (state.probe.kind === "artist") { banner("downloadError", "歌手页面请逐个专辑下载"); return; }

    var btn = $("downloadBtn");
    btn.disabled = true;
    btn.textContent = "正在启动…";

    api("/api/download", {
      method: "POST",
      body: JSON.stringify({ url: state.probe.url, quality: state.quality })
    }).then(function (res) {
      state.sessionId = res.id;
      state.lastLogCount = 0;
      hide("infoCard");
      hide("searchCard");
      show("progressCard");
      text("pKind", kindLabel(state.probe.kind));
      text("pName", state.probe.name || "—");
      text("pArtist", state.probe.artist || "—");
      text("pQuality", qualityLabel(state.quality));
      $("logPane").textContent = "";
      setProgress(0, 0, 0, "");
      startPolling();
    }).catch(function (err) {
      banner("downloadError", err.message);
    }).finally(function () {
      btn.disabled = false;
      btn.textContent = "确认下载";
    });
  }

  function setProgress(percent, done, total, current) {
    $("progressBar").style.width = Math.max(0, Math.min(100, percent)) + "%";
    text("pPercent", percent + "%");
    text("pCounts", total > 0 ? done + " / " + total : "—");
    if (current != null) text("pCurrent", current);
  }

  function startPolling() {
    stopPolling();
    state.poll = setInterval(pollSession, 900);
    pollSession();
  }

  function stopPolling() {
    if (state.poll) { clearInterval(state.poll); state.poll = null; }
  }

  function pollSession() {
    if (!state.sessionId) return;
    api("/api/session?id=" + encodeURIComponent(state.sessionId)).then(function (s) {
      setProgress(s.percent, s.processed, s.total, s.currentItem || "");

      var log = s.log || [];
      if (log.length !== state.lastLogCount) {
        var pane = $("logPane");
        pane.textContent = log.join("\n");
        pane.scrollTop = pane.scrollHeight;
        state.lastLogCount = log.length;
      }

      if (s.status === "completed" || s.status === "failed") {
        stopPolling();
        showDone(s);
      }
    }).catch(function () {});
  }

  /* ---------- step 3: done ---------- */

  function showDone(s) {
    var ok = s.status === "completed";
    var card = $("doneCard");
    card.classList.toggle("failed", !ok);
    hide("progressCard");
    hide("searchCard");
    show("doneCard");

    $("doneIcon").textContent = ok ? "✓" : "!";
    text("doneTitle", ok ? "已下载完成" : "下载未完成");

    // Show the codec the engine actually negotiated: a .m4a file is a
    // container, and only this line proves whether it holds ALAC or AAC.
    var subtitle = ok
      ? "成功 " + s.success + " 首 · 保存于 " + (s.outputDir || "")
      : (s.errorText || "下载过程中出现错误");
    text("doneSub", subtitle);

    var qbox = $("doneQuality");
    if (ok && s.reportedQuality) {
      qbox.textContent = "实际编码：" + s.reportedQuality;
      qbox.classList.remove("hidden");
    } else {
      qbox.classList.add("hidden");
      qbox.textContent = "";
    }

    var box = $("doneFiles");
    box.innerHTML = "";
    (s.addedTracks || []).forEach(function (t) {
      var row = document.createElement("div");
      row.className = "done-file";

      var name = document.createElement("div");
      name.className = "fname";
      name.textContent = (t.artist ? t.artist + " · " : "") + (t.song || "");

      var p = document.createElement("div");
      p.className = "fpath";
      p.textContent = t.path || "";

      row.appendChild(name);
      row.appendChild(p);
      box.appendChild(row);
    });

    $("revealBtn").disabled = !s.outputDir;
    banner("revealError", "");
  }

  function reveal() {
    banner("revealError", "");
    if (!state.sessionId) return;
    api("/api/session?id=" + encodeURIComponent(state.sessionId)).then(function (s) {
      var target = s.outputDir || (s.addedTracks && s.addedTracks[0] && s.addedTracks[0].path);
      if (!target) { banner("revealError", "没有可打开的位置"); return; }
      return api("/api/reveal", {
        method: "POST",
        body: JSON.stringify({ path: target })
      }).catch(function (err) {
        banner("revealError", err.message);
      });
    }).catch(function () {});
  }

  /* ---------- in-page preview player ---------- */

  var player = {
    audio: null,
    list: [],
    index: -1,
    dragging: false,
    loading: false
  };

  // The preview is played through amdl's own /api/preview endpoint rather than
  // straight from Apple. Apple labels these clips audio/x-m4p (the FairPlay MP4
  // type) even though the bytes are a plain, unencrypted AAC-in-MP4 track, and
  // browsers refuse to decode x-m4p. Proxying lets the server set audio/mp4 and
  // also removes the cross-origin dependency.
  //
  // The token travels as a query parameter because an <audio> element cannot
  // send custom headers, so the usual X-AmDL-Token header is unavailable here.
  function previewProxyURL(raw) {
    return "/api/preview?t=" + encodeURIComponent(TOKEN) + "&u=" + encodeURIComponent(raw);
  }

  // Apple publishes exactly one preview clip per track (AAC, ~30s), so playback
  // always uses it regardless of the selected download quality. The UI says so
  // explicitly instead of implying the quality toggle changes the preview.
  function ensureAudio() {
    if (!player.audio) {
      player.audio = new Audio();
      player.audio.preload = "auto";
      player.audio.addEventListener("timeupdate", onPlayerTime);
      player.audio.addEventListener("loadedmetadata", onPlayerTime);
      player.audio.addEventListener("ended", function () { playerNext(); });
      player.audio.addEventListener("play", function () { text("playerIcon", "\u275A\u275A"); });
      player.audio.addEventListener("pause", function () { text("playerIcon", "\u25B6"); });
      player.audio.addEventListener("error", function () {
        text("playerNote", "该试听片段无法播放");
        text("playerIcon", "\u25B6");
      });
    }
    return player.audio;
  }

  function onPlayerTime() {
    var a = player.audio;
    if (!a) { return; }
    text("playerNow", fmtDuration(a.currentTime * 1000));
    text("playerDur", fmtDuration((isFinite(a.duration) ? a.duration : 0) * 1000));
    if (!player.dragging && a.duration > 0) {
      $("playerRange").value = String(Math.round((a.currentTime / a.duration) * 1000));
    }
  }

  // playTrack starts a preview. list/index describe the queue so next/prev work.
  function playTrack(list, index) {
    var t = list[index];
    if (!t) { return; }

    player.list = list;
    player.index = index;
    openPlayer();
    text("playerTitle", t.name || "");
    $("playerRange").value = "0";
    text("playerNow", "0:00");
    text("playerDur", "0:00");
    markActiveTrack();

    if (!t.previewUrl) {
      text("playerNote", "该曲目没有试听片段");
      return;
    }

    var a = ensureAudio();
    a.src = previewProxyURL(t.previewUrl);
    text("playerNote", "音质选择仅影响下载文件");
    a.play().catch(function () {
      // The proxy URL is a plain same-origin file, so a rejection here means
      // the click was not treated as a user gesture.
      text("playerNote", "请点击播放按钮开始试听");
      text("playerIcon", "\u25B6");
    });
  }

  function openPlayer() {
    $("player").classList.remove("hidden");
    document.body.classList.add("has-player");
  }

  function closePlayer() {
    if (player.audio) { player.audio.pause(); }
    $("player").classList.add("hidden");
    document.body.classList.remove("has-player");
    player.index = -1;
    markActiveTrack();
  }

  function togglePlay() {
    var a = player.audio;
    if (!a || !a.src) {
      // Nothing queued yet: start from the first previewable track.
      for (var i = 0; i < player.list.length; i++) {
        if (player.list[i].previewUrl) { playTrack(player.list, i); return; }
      }
      return;
    }
    if (a.paused) { a.play().catch(function () {}); } else { a.pause(); }
  }

  function playerNext() { stepTrack(1); }
  function playerPrev() { stepTrack(-1); }

  function stepTrack(delta) {
    var list = player.list;
    if (!list.length) { return; }
    var i = player.index;
    for (var n = 0; n < list.length; n++) {
      i = (i + delta + list.length) % list.length;
      if (list[i].previewUrl) { playTrack(list, i); return; }
    }
  }

  function markActiveTrack() {
    var rows = document.querySelectorAll("#trackList li");
    for (var i = 0; i < rows.length; i++) {
      var on = i === player.index;
      rows[i].classList.toggle("active", on);
      var btn = rows[i].querySelector(".track-play");
      if (btn) {
        btn.classList.toggle("playing", on);
        btn.textContent = on ? "\u275A\u275A" : "\u25B6";
      }
    }
  }

  /* ---------- views (sidebar navigation) ---------- */

  var currentView = "download";

  function showView(name) {
    currentView = name;
    var views = document.querySelectorAll(".view");
    for (var i = 0; i < views.length; i++) {
      views[i].classList.toggle("hidden", views[i].id !== "view-" + name);
    }
    var items = document.querySelectorAll("#sideNav .nav-item");
    for (var j = 0; j < items.length; j++) {
      items[j].classList.toggle("active", items[j].dataset.view === name);
    }
    // Each view refreshes on entry so it never shows stale data.
    if (name === "tasks" || name === "history") { loadJobs(); }
    if (name === "settings") { loadSettingsPage(); }
    if (name === "about") { loadAbout(); }
  }

  function fmtClock(iso) {
    if (!iso) { return ""; }
    var d = new Date(iso);
    if (isNaN(d.getTime())) { return ""; }
    var p = function (n) { return (n < 10 ? "0" : "") + n; };
    return d.getFullYear() + "-" + p(d.getMonth() + 1) + "-" + p(d.getDate()) +
      " " + p(d.getHours()) + ":" + p(d.getMinutes());
  }

  function statusLabel(st) {
    return ({ running: "进行中", queued: "排队中", completed: "已完成", failed: "失败" })[st] || st;
  }

  /* ---------- tasks & history ---------- */

  var jobs = { active: [], history: [], timer: null };

  function loadJobs() {
    api("/api/tasks").then(function (res) {
      jobs.active = res.active || [];
      jobs.history = res.history || [];
      renderTasks();
      renderHistory();
      updateTaskBadge();
    }).catch(function () {});
  }

  function updateTaskBadge() {
    var n = jobs.active.length;
    var badge = $("taskBadge");
    if (n > 0) { badge.textContent = String(n); badge.classList.remove("hidden"); }
    else { badge.classList.add("hidden"); }
  }

  function jobRow(j, opts) {
    var el = document.createElement("div");
    el.className = "job";

    var top = document.createElement("div");
    top.className = "job-top";

    var left = document.createElement("div");
    left.style.minWidth = "0";
    var name = document.createElement("div");
    name.className = "job-name";
    name.textContent = (j.artist ? j.artist + " · " : "") + (j.name || j.kind || "任务");
    left.appendChild(name);

    var meta = document.createElement("div");
    meta.className = "job-meta";
    var bits = [];
    if (j.quality) { bits.push(qualityLabel(j.quality)); }
    if (j.reportedQuality) { bits.push(j.reportedQuality); }
    if (j.startedAt) { bits.push(fmtClock(j.startedAt)); }
    if (j.total > 0) { bits.push(j.processed + " / " + j.total); }
    if (j.errorText && j.status === "failed") { bits.push(j.errorText); }
    meta.textContent = bits.join(" · ");
    left.appendChild(meta);
    top.appendChild(left);

    var badge = document.createElement("span");
    badge.className = "job-status " + j.status;
    badge.textContent = statusLabel(j.status);
    top.appendChild(badge);

    el.appendChild(top);

    if (j.status === "running" || j.status === "queued") {
      var bar = document.createElement("div");
      bar.className = "job-bar";
      var fill = document.createElement("i");
      fill.style.width = Math.max(0, Math.min(100, j.percent || 0)) + "%";
      bar.appendChild(fill);
      el.appendChild(bar);
    }

    var actions = document.createElement("div");
    actions.className = "job-actions";

    if (j.outputDir) {
      var open = document.createElement("button");
      open.className = "btn ghost small";
      open.textContent = "打开位置";
      open.addEventListener("click", function () { revealPath(j.outputDir); });
      actions.appendChild(open);
    }
    if (opts && opts.deletable) {
      var del = document.createElement("button");
      del.className = "btn ghost small";
      del.textContent = "删除";
      del.addEventListener("click", function () {
        api("/api/history/delete?id=" + encodeURIComponent(j.id), { method: "POST" })
          .then(loadJobs)
          .catch(function (err) { alert(err.message); });
      });
      actions.appendChild(del);
    }
    if (actions.children.length) { el.appendChild(actions); }

    return el;
  }

  function renderTasks() {
    var box = $("taskList");
    box.innerHTML = "";
    if (!jobs.active.length) {
      box.innerHTML = '<p class="muted">暂无进行中的任务。</p>';
      return;
    }
    jobs.active.forEach(function (j) { box.appendChild(jobRow(j, { deletable: false })); });
  }

  function renderHistory() {
    var box = $("historyList");
    box.innerHTML = "";
    text("historyCount", jobs.history.length ? "共 " + jobs.history.length + " 条记录" : "暂无记录");
    // Clearing an empty list is a no-op, so the button reflects that.
    $("clearHistoryBtn").disabled = jobs.history.length === 0;
    if (!jobs.history.length) {
      box.innerHTML = '<p class="muted">还没有下载记录。</p>';
      return;
    }
    jobs.history.forEach(function (j) { box.appendChild(jobRow(j, { deletable: true })); });
  }

  // The task view polls while it is visible so progress stays live.
  function startJobPolling() {
    if (jobs.timer) { return; }
    jobs.timer = setInterval(function () {
      if (currentView === "tasks" || currentView === "history") { loadJobs(); }
    }, 1500);
  }

  /* ---------- settings ---------- */

  function loadSettingsPage() {
    api("/api/settings").then(function (s) {
      text("setLite", s.liteServer || "—");
      $("setLite").textContent = s.liteServer || "—";
      $("setHistoryFile").textContent = s.historyFile || "—";
      $("setStateFile").textContent = s.stateFile || "—";
    }).catch(function () {});
    api("/api/about").then(function (a) {
      $("setAlac").textContent = (a.paths && a.paths.alac) || "—";
      $("setAtmos").textContent = (a.paths && a.paths.atmos) || "—";
      $("setAac").textContent = (a.paths && a.paths.aac) || "—";
      $("setMv").textContent = (a.paths && a.paths.mv) || "—";
      text("setLite", a.liteServer || "—");
      var q = a.quality || {};
      text("setLogin", q.loggedIn ? "已登录" : "未登录");
      text("setRegions", (q.regions && q.regions.length) ? q.regions.join(", ") : "—");
      text("setAlacMax", q.alacMax ? q.alacMax + " Hz" : "—");
      text("setAtmosMax", q.atmosMax ? q.atmosMax + " kbps" : "—");
      text("setAacType", q.aacType || "—");
      text("setLosslessOnly", q.losslessOnly ? "是" : "否");
      text("setAlacFix", (q.alacFix ? "修复" : "不修复") + " / " + (q.alacVerify ? "校验" : "不校验"));
    }).catch(function () {});
  }

  /* ---------- about ---------- */

  function loadAbout() {
    api("/api/about").then(function (a) {
      text("aboutVersion", versionText(a));
      text("sideVersion", shortVersionText(a));
      text("aboutCommit", a.commit || "—");
      text("aboutBuild", a.buildDate || "—");
      text("aboutGo", a.goVersion || "—");
    }).catch(function () {});
  }

  // versionText renders the full human-readable version for the about view.
  function versionText(a) {
    if (a.dev) { return "开发版（源码构建）"; }
    return "v" + a.version;
  }

  // shortVersionText is the compact form shown in the sidebar footer.
  function shortVersionText(a) {
    return a.dev ? "v" + a.version + " (dev)" : "v" + a.version;
  }

  // Load the version into the sidebar immediately, independent of the view.
  function loadVersionBadge() {
    api("/api/about").then(function (a) {
      text("sideVersion", shortVersionText(a));
    }).catch(function () {});
  }

  function revealPath(path) {
    if (!path) { return; }
    api("/api/reveal", { method: "POST", body: JSON.stringify({ path: path }) })
      .catch(function (err) { alert(err.message); });
  }

  /* ---------- wiring ---------- */

  function reset() {
    stopPolling();
    state.probe = null;
    state.quality = null;
    state.sessionId = null;
    state.lastLogCount = 0;
    $("urlInput").value = "";
    hide("infoCard");
    hide("progressCard");
    hide("doneCard");
    show("inputCard");
    show("searchCard");
    banner("probeError", "");
    banner("downloadError", "");
    banner("revealError", "");
    $("urlInput").focus();
  }

  $("probeBtn").addEventListener("click", probe);
  $("urlInput").addEventListener("keydown", function (e) {
    if (e.key === "Enter") probe();
  });
  $("searchBtn").addEventListener("click", function () { runSearch(0); });
  $("searchInput").addEventListener("keydown", function (e) {
    if (e.key === "Enter") runSearch(0);
  });
  $("prevPageBtn").addEventListener("click", function () {
    runSearch(Math.max(0, search.offset - search.limit));
  });
  $("nextPageBtn").addEventListener("click", function () {
    runSearch(search.offset + search.limit);
  });
  var tabs = document.querySelectorAll("#searchTabs .tab");
  for (var i = 0; i < tabs.length; i++) {
    tabs[i].addEventListener("click", function () {
      for (var j = 0; j < tabs.length; j++) tabs[j].classList.remove("active");
      this.classList.add("active");
      search.type = this.dataset.type;
      // A new type means a new result set, so restart at the first page.
      if (search.q) { runSearch(0); }
    });
  }
  $("downloadBtn").addEventListener("click", startDownload);
  $("cancelBtn").addEventListener("click", reset);
  $("againBtn").addEventListener("click", reset);
  $("revealBtn").addEventListener("click", reveal);
  $("toggleTracks").addEventListener("click", function () {
    var list = $("trackList");
    var open = list.classList.toggle("hidden");
    $("toggleTracks").textContent = open ? "展开" : "收起";
  });

  $("loginBtn").addEventListener("click", function () { openLogin(true); });
  $("loginSubmitBtn").addEventListener("click", submitLogin);
  $("loginLaterBtn").addEventListener("click", closeLogin);
  // Enter in any credential field submits the form.
  ["loginUser", "loginPass", "loginCode"].forEach(function (id) {
    $(id).addEventListener("keydown", function (e) {
      if (e.key === "Enter") { e.preventDefault(); submitLogin(); }
    });
  });

  $("playerToggle").addEventListener("click", togglePlay);
  $("playerNext").addEventListener("click", playerNext);
  $("playerPrev").addEventListener("click", playerPrev);
  $("playerClose").addEventListener("click", closePlayer);
  $("playerRange").addEventListener("input", function () {
    player.dragging = true;
    var a = player.audio;
    if (a && a.duration > 0) {
      text("playerNow", fmtDuration((this.value / 1000) * a.duration * 1000));
    }
  });
  $("playerRange").addEventListener("change", function () {
    var a = player.audio;
    if (a && a.duration > 0) { a.currentTime = (this.value / 1000) * a.duration; }
    player.dragging = false;
  });
  // Space toggles playback, unless the user is typing in a field.
  document.addEventListener("keydown", function (e) {
    if (e.code !== "Space") { return; }
    var el = e.target;
    if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.isContentEditable)) { return; }
    if ($("player").classList.contains("hidden")) { return; }
    e.preventDefault();
    togglePlay();
  });

  // Sidebar navigation.
  var navItems = document.querySelectorAll("#sideNav .nav-item");
  for (var n = 0; n < navItems.length; n++) {
    navItems[n].addEventListener("click", function () { showView(this.dataset.view); });
  }
  $("sideVersion").addEventListener("click", function () { showView("about"); });
  $("clearHistoryBtn").addEventListener("click", function () {
    if (!jobs.history.length) { return; }
    if (!confirm("确定清空全部历史记录？正在进行的任务不受影响。")) { return; }
    api("/api/history/clear", { method: "POST" })
      .then(loadJobs)
      .catch(function (err) { alert(err.message); });
  });
  $("reloadSettingsBtn").addEventListener("click", loadSettingsPage);
  $("connReloadBtn").addEventListener("click", reloadPage);

  // A periodic liveness check means a dropped server is noticed even when the
  // user is idle, not only on their next action.
  setInterval(function () {
    if (conn.down) { pingService(); }
  }, 5000);

  checkService();
  loadVersionBadge();
  startJobPolling();
  $("urlInput").focus();
})();`
