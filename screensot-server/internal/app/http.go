package app

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const userIndexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>屏幕查看</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; }
    label { display: block; margin-bottom: 8px; }
    input { padding: 6px 8px; width: 260px; }
    button { margin-top: 12px; padding: 6px 12px; }
    .hint { margin-top: 12px; color: #666; }
  </style>
</head>
<body>
  <h2>输入邀请码查看屏幕</h2>
  <form method="post" action="/">
    <label>邀请码</label>
    <input name="invite_code" placeholder="请输入邀请码" required />
    <div>
      <button type="submit">查看</button>
    </div>
  </form>
  <div class="hint">提示：需要已有客户端连接到服务端。</div>
</body>
</html>`

const adminPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>邀请码管理后台</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; }
    .row { margin-bottom: 12px; }
    input { padding: 6px 8px; }
    button { padding: 6px 12px; margin-right: 8px; }
    table { border-collapse: collapse; width: 100%; margin-top: 12px; }
    th, td { border: 1px solid #ddd; padding: 6px 8px; text-align: left; }
    th { background: #f6f6f6; }
    .error { color: #b00020; }
  </style>
</head>
<body>
  <h2>邀请码管理后台</h2>
  <div class="row">
    <label>Admin Token：</label>
    <input id="token" type="password" placeholder="请输入 ADMIN_TOKEN" />
    <button onclick="saveToken()">保存</button>
    <span id="tokenHint"></span>
  </div>
  <div class="row">
    <label>生成邀请码</label>
    <input id="ttlHours" type="number" value="24" /> 小时
    <input id="note" placeholder="备注（可选）" />
    <button onclick="createInvite()">生成</button>
    <span id="createResult"></span>
  </div>
  <div class="row">
    <label>操作邀请码</label>
    <input id="code" placeholder="输入邀请码" />
    <button onclick="revokeInvite()">撤销</button>
    <button onclick="resetBinding()">重置绑定</button>
  </div>
  <div class="row">
    <button onclick="loadInvites()">刷新列表</button>
    <span id="error" class="error"></span>
  </div>
  <table>
    <thead>
      <tr>
        <th>ID</th>
        <th>邀请码</th>
        <th>过期时间</th>
        <th>已撤销</th>
        <th>绑定设备</th>
        <th>绑定时间</th>
        <th>最近使用</th>
        <th>备注</th>
      </tr>
    </thead>
    <tbody id="list"></tbody>
  </table>

<script>
const tokenKey = "admin_token";
function getQueryToken() {
  const qs = new URLSearchParams(window.location.search);
  return qs.get("token") || "";
}
function getToken() {
  return localStorage.getItem(tokenKey) || "";
}
function saveToken() {
  const v = document.getElementById("token").value.trim();
  localStorage.setItem(tokenKey, v);
  document.getElementById("tokenHint").innerText = v ? "已保存" : "未设置";
  if (v) {
    loadInvites();
  }
}
function authHeaders() {
  const token = getToken();
  return {
    "Content-Type": "application/json",
    "X-Admin-Token": token
  };
}
function fmt(ts) {
  if (!ts) return "";
  const d = new Date(ts * 1000);
  return d.toLocaleString();
}
async function createInvite() {
  clearError();
  const ttlHours = parseInt(document.getElementById("ttlHours").value, 10) || 24;
  const ttl = ttlHours * 3600;
  const note = document.getElementById("note").value;
  const res = await fetch("/admin/invites", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ttl_seconds: ttl, note})
  });
  if (!res.ok) {
    return showError(await res.text());
  }
  const data = await res.json();
  document.getElementById("createResult").innerText = "邀请码: " + data.invite_code + "，过期：" + fmt(data.exp_at);
  loadInvites();
}
async function loadInvites() {
  clearError();
  const res = await fetch("/admin/invites", { headers: authHeaders() });
  if (!res.ok) {
    return showError(await res.text());
  }
  const list = await res.json();
  const tbody = document.getElementById("list");
  tbody.innerHTML = "";
  list.forEach(item => {
    const tr = document.createElement("tr");
    tr.innerHTML =
      "<td>" + item.id + "</td>" +
      "<td>" + (item.invite_code || "") + "</td>" +
      "<td>" + fmt(item.exp_at) + "</td>" +
      "<td>" + (item.revoked ? "是" : "否") + "</td>" +
      "<td>" + (item.bound_device_id || "") + "</td>" +
      "<td>" + fmt(item.bound_at) + "</td>" +
      "<td>" + fmt(item.last_seen_at) + "</td>" +
      "<td>" + (item.note || "") + "</td>";
    tbody.appendChild(tr);
  });
}
async function revokeInvite() {
  clearError();
  const code = document.getElementById("code").value.trim();
  if (!code) return showError("请输入邀请码");
  const res = await fetch("/admin/invites/revoke", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({invite_code: code})
  });
  if (!res.ok) return showError(await res.text());
  loadInvites();
}
async function resetBinding() {
  clearError();
  const code = document.getElementById("code").value.trim();
  if (!code) return showError("请输入邀请码");
  const res = await fetch("/admin/invites/reset-binding", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({invite_code: code})
  });
  if (!res.ok) return showError(await res.text());
  loadInvites();
}
function showError(msg) {
  document.getElementById("error").innerText = msg;
}
function clearError() {
  document.getElementById("error").innerText = "";
}
const urlToken = getQueryToken();
if (urlToken) {
  localStorage.setItem(tokenKey, urlToken);
}
document.getElementById("token").value = getToken();
document.getElementById("tokenHint").innerText = getToken() ? "已保存" : "未设置";
if (getToken()) {
  loadInvites();
}
</script>
</body>
</html>`

// startHTTPServer 注册路由并启动 HTTP 服务
func (a *App) startHTTPServer() {
	http.HandleFunc("/", a.handleIndex)
	http.HandleFunc("/one", a.handleOne)
	http.HandleFunc("/admin", a.handleAdminPage)
	http.HandleFunc("/admin/invites", a.handleAdminInvites)
	http.HandleFunc("/admin/invites/revoke", a.handleAdminRevoke)
	http.HandleFunc("/admin/invites/reset-binding", a.handleAdminResetBinding)
	if err := http.ListenAndServe(":8848", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		inviteCode := strings.TrimSpace(r.FormValue("invite_code"))
		if err := a.invites.ValidateInvite(inviteCode); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		a.renderCapture(w, r, inviteCode)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, userIndexHTML)
}

func (a *App) handleOne(w http.ResponseWriter, r *http.Request) {
	inviteCode := strings.TrimSpace(r.URL.Query().Get("invite_code"))
	if inviteCode == "" {
		http.Error(w, "invite_code required", http.StatusUnauthorized)
		return
	}
	if err := a.invites.ValidateInvite(inviteCode); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	a.renderCapture(w, r, inviteCode)
}

func (a *App) renderCapture(w http.ResponseWriter, r *http.Request, inviteCode string) {
	// 诊断：打印配置摘要，确认运行期可见 key/baseURL/模板路径
	fmt.Fprintf(os.Stderr, "handleOne: models=%v baseURL=%s keylen=%d tpl=%s\n", a.cfg.Models, a.cfg.SiliconflowBaseURL, len(a.cfg.SiliconflowAPIKey), a.cfg.TemplatePath)

	// 支持两种模式：mode=capture 仅截屏；mode=analyze 截屏并识别（默认）
	mode := r.URL.Query().Get("mode")
	analyze := (mode == "" || mode == "analyze")

	a.clientsMutex.Lock()
	numClients := len(a.clients)
	a.clientsMutex.Unlock()
	if numClients == 0 {
		http.Error(w, "No connected clients", http.StatusBadRequest)
		return
	}

	a.sendCaptureCommandToClients()

	// 等待所有客户端的响应
	var allResponses []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func() {
			defer wg.Done()
			select {
			case responseStr := <-a.responseCollector:
				mu.Lock()
				allResponses = append(allResponses, responseStr)
				mu.Unlock()
			case <-time.After(10 * time.Second):
				fmt.Println("Timeout waiting for client response")
			}
		}()
	}
	wg.Wait()

	// 根据模式决定是否进行识别
	var analyses []ImageEntry
	if analyze {
		timeoutSeconds := a.cfg.VisionTimeoutSeconds
		if timeoutSeconds <= 0 {
			timeoutSeconds = 120
		}
		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSeconds)*time.Second)
		defer cancel()
		analyses = a.analyzeImages(ctx, allResponses)
		// 识别后缓存为“最近一次已识别”
		a.setLastAnalyses(analyses)
	} else {
		// 仅截屏模式：合并“新截图的 Base64”与“上一次识别结果的 ModelAnswers”，保留既有识别
		last := a.getLastAnalyses()
		analyses = make([]ImageEntry, len(allResponses))
		for i := range allResponses {
			analyses[i] = ImageEntry{Base64: allResponses[i]}
			if i < len(last) && len(last[i].ModelAnswers) > 0 {
				analyses[i].ModelAnswers = append([]ModelAnswer(nil), last[i].ModelAnswers...)
			}
		}
	}

	// HTML 渲染：模板外置
	type PageData struct {
		Items      []ImageEntry
		InviteCode string
	}
	data := PageData{Items: analyses, InviteCode: inviteCode}
	tplBytes, err := os.ReadFile(a.cfg.TemplatePath)
	if err != nil || len(tplBytes) == 0 {
		// 不存在外部模板时回退到内置模板，确保单文件二进制可运行
		tplBytes = defaultTemplate
	}
	tmpl, err := template.New("result").Parse(string(tplBytes))
	if err != nil {
		http.Error(w, "Internal Server Error: unable to parse template", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Internal Server Error: unable to execute template", http.StatusInternalServerError)
		return
	}
}

type createInviteRequest struct {
	TTLSeconds int64  `json:"ttl_seconds"`
	Note       string `json:"note"`
}

type inviteCodeRequest struct {
	InviteCode string `json:"invite_code"`
}

func (a *App) handleAdminInvites(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req createInviteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		code, expAt, err := a.invites.CreateInvite(req.TTLSeconds, req.Note)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"invite_code": code,
			"exp_at":      expAt,
		})
	case http.MethodGet:
		list, err := a.invites.ListInvites()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, list)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *App) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req inviteCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := a.invites.RevokeInvite(req.InviteCode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleAdminResetBinding(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req inviteCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := a.invites.ResetBinding(req.InviteCode); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *App) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, adminPageHTML)
}

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(a.cfg.AdminToken) == "" {
		http.Error(w, "ADMIN_TOKEN not set", http.StatusInternalServerError)
		return false
	}
	token := r.Header.Get("X-Admin-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token != a.cfg.AdminToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
