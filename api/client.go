// Package api talks to the deployed Next.js app: NextAuth session and the
// file/folder/share endpoints. The desktop app keeps only cookies in memory.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is a session-scoped HTTP client for the Vercel deployment.
type Client struct {
	Base string
	jar  *cookiejar.Jar
	hc   *http.Client
}

// New creates a client for base (for example https://example.vercel.app).
func New(base string) *Client {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	jar, _ := cookiejar.New(nil)
	return &Client{
		Base: base,
		jar:  jar,
		hc:   &http.Client{Jar: jar, Timeout: 60 * time.Second},
	}
}

func (c *Client) endpoint(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return c.Base + path
}

func decode(res *http.Response, out any) error {
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("invalid JSON (%d): %s", res.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	return nil
}

func (c *Client) patchJSON(path string, payload any, out any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPatch, c.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, decode(res, out)
}

func (c *Client) getJSON(path string, out any) (int, error) {
	res, err := c.hc.Get(c.endpoint(path))
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, decode(res, out)
}

func (c *Client) postJSON(path string, payload any, out any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.StatusCode, decode(res, out)
}

// Captcha fetches a new puzzle. The server's SVG uses only lines, circles and
// rectangles, which MyGo's native SVG renderer supports.
func (c *Client) Captcha() (id, svg string, err error) {
	var data struct {
		ID  string `json:"id"`
		SVG string `json:"svg"`
	}
	path := "/api/auth/captcha?t=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	status, err := c.getJSON(path, &data)
	if err != nil {
		return "", "", err
	}
	if status != http.StatusOK {
		return "", "", fmt.Errorf("captcha status %d", status)
	}
	return data.ID, data.SVG, nil
}

// LoginOutcome reports the result of the server-side login guard.
type LoginOutcome struct {
	OK              bool
	Message         string
	Code            string
	CaptchaRequired bool
}

type loginResponse struct {
	OK         bool   `json:"ok"`
	LoginToken string `json:"loginToken"`
	Error      string `json:"error"`
	Code       string `json:"code"`
	Captcha    bool   `json:"captchaRequired"`
}

func fromLoginResponse(res loginResponse) LoginOutcome {
	return LoginOutcome{OK: res.OK, Message: res.Error, Code: res.Code, CaptchaRequired: res.Captcha}
}

// Login runs the same flow as the web login form: server-side guard, a
// one-time ticket, then the NextAuth credentials callback which sets the
// session cookie in this client's jar.
func (c *Client) Login(email, password, captchaID, captchaCode string) (LoginOutcome, error) {
	var first loginResponse
	status, err := c.postJSON("/api/auth/login", map[string]any{
		"email":       email,
		"password":    password,
		"captchaId":   captchaID,
		"captchaCode": captchaCode,
	}, &first)
	if err != nil {
		return LoginOutcome{}, err
	}
	if status != http.StatusOK || !first.OK {
		out := fromLoginResponse(first)
		if out.Message == "" {
			out.Message = fmt.Sprintf("登录失败 (%d)", status)
		}
		return out, nil
	}

	var csrf struct {
		Token string `json:"csrfToken"`
	}
	if _, err := c.getJSON("/api/auth/csrf", &csrf); err != nil {
		return LoginOutcome{}, err
	}
	if csrf.Token == "" {
		return LoginOutcome{}, fmt.Errorf("missing csrf token")
	}

	form := url.Values{}
	form.Set("csrfToken", csrf.Token)
	form.Set("email", email)
	form.Set("password", password)
	form.Set("loginTicket", first.LoginToken)
	form.Set("callbackUrl", c.Base+"/")
	form.Set("json", "true")
	req, err := http.NewRequest(http.MethodPost, c.endpoint("/api/auth/callback/credentials"), strings.NewReader(form.Encode()))
	if err != nil {
		return LoginOutcome{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.hc.Do(req)
	if err != nil {
		return LoginOutcome{}, err
	}
	defer res.Body.Close()
	if err := decode(res, nil); err != nil {
		return LoginOutcome{}, err
	}
	if res.StatusCode >= 400 {
		return LoginOutcome{Message: fmt.Sprintf("建立会话失败 (%d)", res.StatusCode)}, nil
	}

	var session struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if _, err := c.getJSON("/api/auth/session", &session); err != nil {
		return LoginOutcome{}, err
	}
	if session.User.Email == "" {
		return LoginOutcome{Message: "登录失败：会话未建立"}, nil
	}
	return LoginOutcome{OK: true}, nil
}

// Authenticated reports whether the session cookie still works.
func (c *Client) Authenticated() bool {
	_, ok := c.SessionEmail()
	return ok
}

// SessionEmail returns the logged-in user's email, or false when the stored
// session is missing, expired, or invalid.
func (c *Client) SessionEmail() (string, bool) {
	var session struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	status, err := c.getJSON("/api/auth/session", &session)
	if err != nil || status != http.StatusOK || session.User.Email == "" {
		return "", false
	}
	return session.User.Email, true
}

type savedCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Expires  time.Time `json:"expires"`
	Secure   bool      `json:"secure"`
	HttpOnly bool      `json:"httpOnly"`
}

// SaveSession writes the session cookies to a user-only file so a restart
// restores the login. The file contains an encrypted NextAuth JWT, not the
// password, and is created with 0600 permissions.
func (c *Client) SaveSession(path string) error {
	u, err := url.Parse(c.Base)
	if err != nil {
		return err
	}
	items := make([]savedCookie, 0)
	for _, ck := range c.jar.Cookies(u) {
		items = append(items, savedCookie{
			Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path,
			Expires: ck.Expires, Secure: ck.Secure, HttpOnly: ck.HttpOnly,
		})
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadSession restores cookies saved by SaveSession.
func (c *Client) LoadSession(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var items []savedCookie
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	u, err := url.Parse(c.Base)
	if err != nil {
		return err
	}
	cookies := make([]*http.Cookie, 0, len(items))
	for _, it := range items {
		cookies = append(cookies, &http.Cookie{
			Name: it.Name, Value: it.Value, Domain: it.Domain, Path: it.Path,
			Expires: it.Expires, Secure: it.Secure, HttpOnly: it.HttpOnly,
		})
	}
	c.jar.SetCookies(u, cookies)
	return nil
}

// ClearSession removes the persisted session on logout.
func (c *Client) ClearSession(path string) {
	_ = os.Remove(path)
}

// File mirrors the fields the desktop UI needs from GET /api/files.
type File struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Size        int64  `json:"size"`
	MimeType    string `json:"mimeType"`
	CreatedAt   string `json:"createdAt"`
	URL         string `json:"url"`
	DownloadURL string `json:"downloadUrl"`
	ThumbURL    string `json:"thumbnailUrl"`
}

// FilesResponse is the paginated response of GET /api/files.
type FilesResponse struct {
	Files    []File `json:"files"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	HasMore  bool   `json:"hasMore"`
}

// Files lists files in a folder, optionally filtered by name. When sortAsc is
// true the server returns oldest uploads first; otherwise newest first.
func (c *Client) Files(folderID, query string, page, pageSize int, sortAsc bool) (FilesResponse, error) {
	var out FilesResponse
	values := url.Values{}
	if folderID != "" {
		values.Set("folderId", folderID)
	}
	if query != "" {
		values.Set("q", query)
	}
	values.Set("page", strconv.Itoa(page))
	values.Set("pageSize", strconv.Itoa(pageSize))
	if sortAsc {
		values.Set("sort", "asc")
	}
	path := "/api/files?" + values.Encode()
	status, err := c.getJSON(path, &out)
	if err != nil {
		return out, err
	}
	if status != http.StatusOK {
		return out, fmt.Errorf("files status %d", status)
	}
	return out, nil
}

// Folder is a row from GET /api/folders.
type Folder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	FileCount int    `json:"fileCount"`
	CreatedAt string `json:"createdAt"`
}

// Folders lists sub-folders of parent (empty means root).
func (c *Client) Folders(parent string) ([]Folder, error) {
	var folders []Folder
	path := "/api/folders"
	if parent != "" {
		path += "?parentId=" + url.QueryEscape(parent)
	}
	status, err := c.getJSON(path, &folders)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("folders status %d", status)
	}
	return folders, nil
}

// AllFolders walks the folder tree from the root for the pinned-folder picker.
func (c *Client) AllFolders() ([]Folder, error) {
	var all []Folder
	queue := []string{""}
	visited := map[string]bool{}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		children, err := c.Folders(parent)
		if err != nil {
			return all, err
		}
		for _, f := range children {
			if visited[f.ID] {
				continue
			}
			visited[f.ID] = true
			all = append(all, f)
			queue = append(queue, f.ID)
		}
	}
	return all, nil
}

// CreateFolder creates a sub-folder.
func (c *Client) CreateFolder(name, parent string) error {
	payload := map[string]any{"name": name}
	if parent != "" {
		payload["parentId"] = parent
	}
	status, err := c.postJSON("/api/folders", payload, nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("create folder status %d", status)
	}
	return nil
}

// CreateFile records an uploaded object in the database.
func (c *Client) CreateFile(name, key string, size int64, mimeType, folderID string) error {
	payload := map[string]any{
		"name": name, "key": key, "size": size, "mimeType": mimeType,
	}
	if folderID != "" {
		payload["folderId"] = folderID
	}
	status, err := c.postJSON("/api/files", payload, nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("create file status %d", status)
	}
	return nil
}

// MoveFile moves a file to a folder; empty folderID means the root.
func (c *Client) MoveFile(id, folderID string) error {
	status, err := c.patchJSON("/api/files", map[string]any{
		"id": id, "folderId": folderID,
	}, nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("move status %d", status)
	}
	return nil
}

// DeleteFile removes a file (and its object) by ID.
func (c *Client) DeleteFile(id string) error {
	req, reqErr := http.NewRequest(http.MethodDelete, c.endpoint("/api/files?id="+url.QueryEscape(id)), nil)
	if reqErr != nil {
		return reqErr
	}
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("delete status %d", res.StatusCode)
	}
	return nil
}

// Share creates a share link. expiresIn <= 0 means no expiry.
func (c *Client) Share(fileID string, expiresIn int) (string, error) {
	var out struct {
		URL string `json:"url"`
	}
	status, err := c.postJSON("/api/share", map[string]any{
		"fileId": fileID, "expiresIn": expiresIn,
	}, &out)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("share status %d", status)
	}
	return out.URL, nil
}

// UploadTicket is the signed upload policy from the server. The key and the
// upload host are both decided server-side from QINIU_ZONE.
type UploadTicket struct {
	Token     string `json:"token"`
	Key       string `json:"key"`
	UploadURL string `json:"uploadUrl"`
}

// UploadTicket asks the server to sign a Qiniu upload policy.
func (c *Client) UploadTicket(fileName, folderID string) (UploadTicket, error) {
	var out UploadTicket
	payload := map[string]any{"fileName": fileName}
	if folderID != "" {
		payload["folderId"] = folderID
	}
	status, err := c.postJSON("/api/files/upload", payload, &out)
	if err != nil {
		return out, err
	}
	if status != http.StatusOK {
		return out, fmt.Errorf("upload token status %d", status)
	}
	if out.Token == "" || out.Key == "" {
		return out, fmt.Errorf("incomplete upload ticket")
	}
	if out.UploadURL == "" {
		out.UploadURL = "https://upload.qiniup.com"
	}
	return out, nil
}

// FetchAsset downloads a signed file URL for in-app preview.
func FetchAsset(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("下载预览失败 (%d)", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

type progressWriter struct {
	w        io.Writer
	done     int64
	total    int64
	lastPct  int
	onChange func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.onChange != nil {
		pct := 0
		if p.total > 0 {
			pct = int(float64(p.done) * 100 / float64(p.total))
		}
		if pct >= p.lastPct+2 || pct >= 100 {
			p.lastPct = pct
			p.onChange(p.done, p.total)
		}
	}
	return n, err
}

// DownloadToFile streams a signed URL to dest, reporting coarse progress.
// The file is written to a .part file first and renamed on success.
func DownloadToFile(rawURL, dest string, onProgress func(done, total int64)) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	res, err := client.Get(rawURL)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return fmt.Errorf("下载失败 (%d)", res.StatusCode)
	}
	out, err := os.Create(dest + ".part")
	if err != nil {
		return err
	}
	pw := &progressWriter{w: out, total: res.ContentLength, onChange: onProgress}
	_, copyErr := io.Copy(pw, res.Body)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dest + ".part")
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return os.Rename(dest+".part", dest)
}

type progressReader struct {
	r         io.Reader
	sent      int64
	total     int64
	last      int
	onPercent func(int)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.sent += int64(n)
	if p.total > 0 && p.onPercent != nil {
		pct := int(float64(p.sent) * 100 / float64(p.total))
		if pct >= p.last+5 || pct >= 100 {
			p.last = pct
			p.onPercent(pct)
		}
	}
	return n, err
}

// UploadToQiniu sends the bytes to the upload host returned by the server.
// onPercent, when given, receives coarse progress (0-100) as bytes are sent.
func (c *Client) UploadToQiniu(uploadURL, token, key, fileName string, data []byte, onPercent ...func(int)) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("token", token)
	_ = w.WriteField("key", key)
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	var body io.Reader = &buf
	if len(onPercent) > 0 && onPercent[0] != nil {
		body = &progressReader{r: &buf, total: int64(buf.Len()), onPercent: onPercent[0]}
	}
	req, err := http.NewRequest(http.MethodPost, uploadURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("upload status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
