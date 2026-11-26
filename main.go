package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"nongma-drive/api"
)

const defaultBaseURL = "http://localhost:3000"

// app holds long-lived state. Views rebuild every frame from it; no element
// values are ever stored here.
type app struct {
	win         *mygo.Window
	api         *api.Client
	sessionFile string

	screen string // "login" | "files"
	base   string
	email  string
	pass   string
	code   string

	captchaID  string
	captchaSVG string
	loginError string
	busy       bool
	notice     string
	noticeKind string // "", "info", "error", "success"
	noticeAt   time.Time

	menuOpen bool
	menuID   string

	confirmLogout   bool
	settings        appSettings
	settingsPath    string
	settingsOpen    bool
	dirDraft        string
	limitDraft      string
	allFolders      []api.Folder
	foldersLoading  bool
	expanded        map[string]bool
	folderGrid      ui.GridState
	downloads       map[string]*downloadTask
	downloadQueue   []*downloadTask
	activeDownloads int

	preview        *api.File
	previewImage   ui.ImageSource
	previewLoading bool
	previewStage   string
	previewErr     string

	restoring  bool
	uploading  bool
	uploadPct  int
	uploadName string

	uploads      []*uploadTask
	uploadList   ui.ListState
	uploadCenter bool
	uploadFilter string

	viewMode string // "list" | "grid"
	sortAsc  bool
	grid     ui.GridState

	query            string
	current          string
	crumbs           []api.Folder
	files            []api.File
	folders          []api.Folder
	list             ui.ListState
	loading          bool
	loadError        string
	createFolderOpen bool
	folderNameDraft  string
	shareURL         string
}

type appSettings struct {
	DownloadDir  string `json:"downloadDir"`
	MaxDownloads int    `json:"maxDownloads"`
}

type downloadTask struct {
	ID      string
	URL     string
	Name    string
	Path    string
	Status  string // downloading | done | error
	Done    int64
	Total   int64
	Percent int
	Err     string
}

type uploadTask struct {
	ID      string
	Path    string
	Name    string
	Size    int64
	Status  string // queued | uploading | done | error
	Percent int
	Err     string
}

type fileCategory struct {
	Key        string
	Label      string
	Extensions []string
	MIMETypes  []string
}

var uploadCategories = []fileCategory{
	{Key: "", Label: "全部", Extensions: []string{"*"}},
	{Key: "archive", Label: "压缩包", Extensions: []string{"zip", "rar", "7z", "tar", "gz", "bz2"}},
	{Key: "image", Label: "图片", Extensions: []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "heic"}, MIMETypes: []string{"image/*"}},
	{Key: "video", Label: "视频", Extensions: []string{"mp4", "mov", "avi", "mkv", "webm"}, MIMETypes: []string{"video/*"}},
	{Key: "document", Label: "文档", Extensions: []string{"pdf", "doc", "docx", "txt", "md", "xls", "xlsx", "csv", "ppt", "pptx"}, MIMETypes: []string{"application/pdf", "text/*"}},
}

func (a *app) setAPI() { a.api = api.New(a.base) }

func (a *app) async(fn func()) {
	go func() {
		fn()
		a.win.Update(func() {})
	}()
}

func (a *app) setError(msg string) {
	a.notice, a.noticeKind, a.noticeAt = msg, "error", time.Now()
}

func (a *app) setSuccess(msg string) {
	a.notice, a.noticeKind, a.noticeAt = msg, "success", time.Now()
}

func (a *app) setInfo(msg string) {
	a.notice, a.noticeKind, a.noticeAt = msg, "info", time.Now()
}

// ---------- design tokens ----------

// panel is real Liquid Glass: the shader refracts whatever was painted
// underneath it earlier in this frame. No opaque background, no dropped
// shadow — the material draws its own lit hairline rim.
func panel(c *ui.Context) ui.Element {
	return ui.Box(c).Radius(22).Material(glass.Glass{})
}

// softPanel is a clearer glass for large panes such as the sidebar.
func softPanel(c *ui.Context) ui.Element {
	return ui.Box(c).Radius(20).Material(glass.Glass{Clarity: 0.55})
}

// backdrop paints colorful shapes at the bottom of the frame for the glass
// panels above to refract. ZIndex keeps it behind every interactive element.
func backdrop(c *ui.Context) {
	ui.Box(c).Absolute().Top(-60).Left(240).Width(620).Height(620).ZIndex(-1).
		RadialGradient(ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(56, 189, 248, 0.55), At: 0},
				{Color: ui.RGBA(56, 189, 248, 0.16), At: 0.45},
				{Color: ui.Transparent, At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.5}, Radius: 310,
		})
	ui.Box(c).Absolute().Top(180).Right(-220).Width(760).Height(760).ZIndex(-1).
		RadialGradient(ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(167, 139, 250, 0.48), At: 0},
				{Color: ui.RGBA(167, 139, 250, 0.12), At: 0.5},
				{Color: ui.Transparent, At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.5}, Radius: 380,
		})
	ui.Box(c).Absolute().Top(260).Left(480).Width(560).Height(560).ZIndex(-1).
		RadialGradient(ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(45, 212, 191, 0.28), At: 0},
				{Color: ui.Transparent, At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.5}, Radius: 280,
		})
	ui.Box(c).Absolute().Bottom(-260).Left(280).Width(700).Height(700).ZIndex(-1).
		RadialGradient(ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(244, 114, 182, 0.34), At: 0},
				{Color: ui.Transparent, At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.5}, Radius: 350,
		})
}

func divider(c *ui.Context) {
	c.Theme()
	ui.Box(c).FillWidth().Height(1).Background(c.Theme().Border.Alpha(0.35))
}

func brandMark(c *ui.Context, size float32) {
	t := c.Theme()
	ui.Box(c).Width(size).Height(size).Radius(size * 0.28).
		Background(t.Accent).AlignItems(ui.Center).Justify(ui.Center).
		Children(func() {
			ui.Text(c, "R").FontSize(size * 0.52).Bold().TextColor(t.AccentText)
		})
}

func field(c *ui.Context, label string, value *string, placeholder string, password bool, onChange ...func()) {
	t := c.Theme()
	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
		input := ui.TextInput(c, value)
		if password {
			input.Password()
		}
		e := input.Placeholder(placeholder).Height(36).Radius(9).Material(glass.Glass{})
		if len(onChange) > 0 && onChange[0] != nil {
			e.OnChange(onChange[0])
		}
	})
}

func primary(c *ui.Context, label string, width float32) ui.Element {
	t := c.Theme()
	b := ui.Button(c, label).Height(40).Radius(9).
		Material(glass.Glass{Tint: t.Accent, Interactive: true}).
		TextColor(t.AccentText).Bold()
	if width > 0 {
		b.Width(width)
	}
	return b
}

func ghost(c *ui.Context, label string, height float32) ui.Element {
	return ui.Button(c, label).Height(height).Radius(9).Material(glass.Glass{Interactive: true})
}

// iconButton keeps the icon and label centered as one unit instead of
// relying on a text glyph inside a plain button.
func iconButton(c *ui.Context, icon *ui.SVG, label string, height float32, accent bool) ui.Element {
	t := c.Theme()
	material := glass.Glass{Interactive: true}
	text, iconColor := t.Text, t.TextMuted
	if accent {
		material.Tint, text, iconColor = t.Accent, t.AccentText, t.AccentText
	}
	e := ui.Row(c).Height(height).Padding(0, 11).Gap(7).AlignItems(ui.Center).
		Radius(9).Material(material)
	e.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(iconColor)
		ui.Text(c, label).FontSize(12).TextColor(text)
	})
	return e
}

func iconOnlyButton(c *ui.Context, icon *ui.SVG, size float32) ui.Element {
	e := ui.Row(c).Width(size).Height(size).AlignItems(ui.Center).Justify(ui.Center).
		Radius(size / 2).Material(glass.Glass{Interactive: true})
	e.Children(func() {
		ui.Icon(c, icon).FontSize(15)
	})
	return e
}

// The bare variants are for glass.Group panes: the group paints one glass
// surface behind all of them, so each pane must not fill its own material.
func iconButtonBare(c *ui.Context, icon *ui.SVG, label string, height float32, accent bool) ui.Element {
	t := c.Theme()
	text, iconColor := t.Text, t.TextMuted
	if accent {
		text, iconColor = t.AccentText, t.AccentText
	}
	e := ui.Row(c).Height(height).Padding(0, 11).Gap(7).AlignItems(ui.Center).Radius(9)
	e.Children(func() {
		ui.Icon(c, icon).FontSize(15).TextColor(iconColor)
		ui.Text(c, label).FontSize(12).TextColor(text)
	})
	return e
}

var ringCache sync.Map // key "size-percent" -> *ui.SVG

// ringSVG is a real circle rendered by MyGo's SVG renderer: the progress
// circle is the same circle with a dash offset, starting at 12 o'clock.
func ringSVG(size float32, pct int) *ui.SVG {
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	key := fmt.Sprintf("%d-%d", int(size), pct)
	if cached, ok := ringCache.Load(key); ok {
		return cached.(*ui.SVG)
	}
	center := size / 2
	radius := center - 2
	circumference := 2 * math.Pi * float64(radius)
	offset := circumference * (1 - float64(pct)/100)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g" viewBox="0 0 %g %g">`+
		`<circle cx="%g" cy="%g" r="%g" fill="none" stroke="#4b5563" stroke-width="2.5"/>`+
		`<circle cx="%g" cy="%g" r="%g" fill="none" stroke="#3b82f6" stroke-width="2.5" stroke-linecap="round" `+
		`stroke-dasharray="%.2f" stroke-dashoffset="%.2f" transform="rotate(-90 %g %g)"/></svg>`,
		size, size, size, size,
		center, center, radius,
		center, center, radius,
		circumference, offset, center, center)
	parsed, err := ui.ParseSVG([]byte(svg))
	if err != nil {
		return nil
	}
	ringCache.Store(key, parsed)
	return parsed
}

// progressRing draws the circular download progress next to a percent label.
func progressRing(c *ui.Context, size float32, pct int) {
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		if svg := ringSVG(size, pct); svg != nil {
			ui.Image(c, svg).Width(size).Height(size)
		}
		ui.Text(c, fmt.Sprintf("%d%%", pct)).FontSize(10).TextColor(t.Accent)
	})
}

// downloadIndicator shows an App-Store-style circular progress on a file item.
func (a *app) downloadIndicator(c *ui.Context, id string) {
	task := a.downloads[id]
	if task == nil {
		return
	}
	t := c.Theme()
	switch task.Status {
	case "queued":
		ui.Text(c, "排队中").FontSize(10).TextColor(t.TextMuted)
	case "downloading":
		progressRing(c, 26, task.Percent)
	case "done":
		ui.Text(c, "已下载").FontSize(10).TextColor(ui.Hex("#63dba1"))
	case "error":
		ui.Text(c, "下载失败").FontSize(10).TextColor(ui.Hex("#ff9095"))
	}
}

func iconOnlyButtonBare(c *ui.Context, icon *ui.SVG, size float32) ui.Element {
	e := ui.Row(c).Width(size).Height(size).AlignItems(ui.Center).Justify(ui.Center).Radius(size / 2)
	e.Children(func() {
		ui.Icon(c, icon).FontSize(15)
	})
	return e
}

// ---------- screens ----------

// gridMetrics adapts the grid to the window width: the wider the content
// area, the more columns, with each card keeping a sensible aspect ratio.
func (a *app) gridMetrics() (float32, float32) {
	contentWidth := 1100
	if a.win != nil {
		if w, _ := a.win.ContentSize(); w > 0 {
			contentWidth = w
		}
	}
	// Sidebar + paddings + list-card padding.
	available := float32(contentWidth) - 310
	if available < 420 {
		available = 420
	}
	// Aim for compact Finder-like icons: more columns and rows per screen.
	columns := int(available / 175)
	if columns < 3 {
		columns = 3
	}
	if columns > 9 {
		columns = 9
	}
	minWidth := available/float32(columns) - 12
	if minWidth < 128 {
		minWidth = 128
	}
	if minWidth > 190 {
		minWidth = 190
	}
	height := minWidth*0.82 + 14
	return minWidth, height
}

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	tb := c.TitleBar()
	root := ui.Column(c).Fill().Gap(0)
	// An opaque base keeps the glass material readable; the colored shapes
	// in backdrop() are what the panels actually bend and blur.
	root.Gradient(ui.RGBA(14, 22, 48, 1), ui.RGBA(30, 20, 54, 1), 135)
	root.Children(func() {
		backdrop(c)
		// A slim, draggable title strip; window controls occupy its right edge.
		ui.Row(c).FillWidth().Height(tb.Height+6).
			Padding(0, 14+tb.Right, 0, 14+tb.Left).
			AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Text(c, "资源管家").FontSize(12).TextColor(t.TextMuted).Grow(1)
		})

		switch {
		case a.restoring:
			a.restoreContent(c)
		case a.screen == "files":
			a.filesContent(c)
		default:
			a.loginContent(c)
		}

		if a.confirmLogout {
			a.logoutDialog(c)
		}
		if a.preview != nil {
			a.previewOverlay(c)
		}
		if a.settingsOpen {
			a.settingsDialog(c)
		}
		if a.createFolderOpen {
			a.createFolderDialog(c)
		}
	})
	_ = t
}

func (a *app) logoutDialog(c *ui.Context) {
	t := c.Theme()
	scrim := ui.Box(c.Key("logout-scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(20).Background(ui.RGBA(0, 0, 0, 0.42))
	if scrim.Clicked() {
		a.confirmLogout = false
	}
	ui.Box(c.Key("logout-center")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(21).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		card := ui.Box(c).Width(360).Padding(24).Gap(16).
			Radius(18).Material(glass.Glass{})
		card.Children(func() {
			ui.Text(c, "退出登录？").FontSize(18).Bold()
			ui.Text(c, "退出后需要重新登录才能继续管理文件。").
				FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ghost(c, "取消", 36).Clicked() {
					a.confirmLogout = false
				}
				confirm := ui.Button(c, "确认退出").Height(36).Radius(9).
					Material(glass.Glass{Tint: ui.Hex("#e5484d")}).TextColor(ui.Hex("#ffffff"))
				if confirm.Clicked() {
					a.logout()
				}
			})
		})
	})
}

func (a *app) previewOverlay(c *ui.Context) {
	t := c.Theme()
	scrim := ui.Box(c.Key("preview-scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(20).Background(ui.RGBA(0, 0, 0, 0.5))
	if scrim.Clicked() {
		a.closePreview()
	}
	ui.Box(c.Key("preview-center")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(21).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		card := ui.Box(c).Width(900).Height(650).Padding(18).Gap(10).
			Radius(18).Material(glass.Glass{})
		card.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, a.preview.Name).FontSize(16).Bold().Grow(1)
				if iconOnlyButton(c, iconX, 28).Clicked() {
					a.closePreview()
				}
			})
			if a.previewLoading {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Grow(1).Children(func() {
					ui.Spinner(c)
					ui.Text(c, a.previewStage).FontSize(12).TextColor(t.TextMuted)
				})
			}
			if a.previewErr != "" {
				ui.Text(c, a.previewErr).FontSize(12).TextColor(ui.Hex("#ff9095"))
				if ghost(c, "下载后查看", 34).Clicked() {
					a.downloadFile(*a.preview)
				}
			}
			if a.previewImage != nil {
				ui.Image(c, a.previewImage).Fit(ui.Contain).
					MaxWidthPercent(100).MaxHeightPercent(100).Grow(1)
			}
		})
	})
}

func (a *app) loadAllFolders() {
	a.foldersLoading = true
	a.async(func() {
		folders, err := a.api.AllFolders()
		a.win.Update(func() {
			a.foldersLoading = false
			if err == nil {
				a.allFolders = folders
			} else {
				a.setError("加载文件夹列表失败: " + err.Error())
			}
		})
	})
}

func (a *app) settingsDialog(c *ui.Context) {
	t := c.Theme()
	scrim := ui.Box(c.Key("settings-scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(20).Background(ui.RGBA(0, 0, 0, 0.42))
	if scrim.Clicked() {
		a.settingsOpen = false
	}
	ui.Box(c.Key("settings-center")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(21).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		card := ui.Box(c).Width(460).Padding(24).Gap(14).Radius(18).Material(glass.Glass{})
		card.Children(func() {
			ui.Text(c, "设置").FontSize(18).Bold()
			ui.Column(c).Gap(5).Children(func() {
				ui.Text(c, "下载位置").FontSize(11).TextColor(t.TextMuted)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &a.dirDraft).Placeholder(a.defaultDownloadDir()).
						Height(36).Radius(9).Material(glass.Glass{}).Grow(1)
					if iconOnlyButton(c, iconFolder, 36).Clicked() {
						paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
							Title: "选择下载目录", Directory: true, Parent: a.win,
						})
						if err == nil && len(paths) > 0 {
							a.dirDraft = paths[0]
						}
					}
				})
			})
			field(c, "同时下载任务数（1-10）", &a.limitDraft, "1", false)
			ui.Text(c, "文件会下载到该目录；同名文件自动加序号，不会覆盖。超过任务数的下载会排队。").
				FontSize(11).TextColor(t.TextMuted)

			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ghost(c, "取消", 34).Clicked() {
					a.settingsOpen = false
				}
				if ui.Button(c, "保存").Height(34).Radius(9).
					Material(glass.Glass{Tint: t.Accent, Interactive: true}).
					TextColor(t.AccentText).Clicked() {
					dir := strings.TrimSpace(a.dirDraft)
					if dir != "" {
						a.settings.DownloadDir = dir
					}
					if n, err := strconv.Atoi(strings.TrimSpace(a.limitDraft)); err == nil {
						if n < 1 || n > 10 {
							a.setError("同时下载任务数需在 1-10 之间")
						} else {
							a.settings.MaxDownloads = n
						}
					}
					if err := saveSettings(a.settingsPath, a.settings); err != nil {
						a.setError("保存设置失败: " + err.Error())
					} else {
						a.setSuccess("设置已保存")
						a.pumpDownloads()
					}
					a.settingsOpen = false
				}
			})
		})
	})
}

func (a *app) createFolderDialog(c *ui.Context) {
	t := c.Theme()
	scrim := ui.Box(c.Key("new-folder-scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(20).Background(ui.RGBA(0, 0, 0, 0.42))
	if scrim.Clicked() {
		a.createFolderOpen = false
	}
	ui.Box(c.Key("new-folder-center")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
		ZIndex(21).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		card := ui.Box(c).Width(360).Padding(24).Gap(16).Radius(18).Material(glass.Glass{})
		card.Children(func() {
			ui.Text(c, "新建文件夹").FontSize(18).Bold()
			field(c, "文件夹名称", &a.folderNameDraft, "输入文件夹名称", false)
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if ghost(c, "取消", 36).Clicked() {
					a.createFolderOpen = false
				}
				if ui.Button(c, "创建").Height(36).Radius(9).
					Material(glass.Glass{Tint: t.Accent, Interactive: true}).
					TextColor(t.AccentText).Clicked() {
					name := strings.TrimSpace(a.folderNameDraft)
					if name == "" {
						a.setError("请输入文件夹名称")
					} else {
						a.createFolder(name)
						a.createFolderOpen = false
					}
				}
			})
		})
	})
}

// restoreContent is the first frame shown when a saved session exists. It
// avoids flashing the login form while we verify the session with the server.
func (a *app) restoreContent(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(14).Children(func() {
		brandMark(c, 54)
		ui.Text(c, "资源管家").FontSize(22).Bold()
		ui.Spinner(c)
		ui.Text(c, "正在恢复会话…").FontSize(12).TextColor(t.TextMuted)
	})
}

func (a *app) loginContent(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Padding(20, 32, 32).Children(func() {
		card := panel(c)
		card.Width(430).Padding(26).Gap(11).Children(func() {
			// Brand block.
			ui.Row(c).Gap(13).AlignItems(ui.Center).Children(func() {
				brandMark(c, 42)
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "资源管家").FontSize(24).Bold()
					ui.Text(c, "登录以管理你的文件").FontSize(12).TextColor(t.TextMuted)
				})
			})
			divider(c)

			field(c, "服务地址", &a.base, "https://your-app.vercel.app", false, func() { a.setAPI() })
			field(c, "邮箱", &a.email, "name@example.com", false)
			field(c, "密码", &a.pass, "请输入密码", true)

			// Captcha is not shown by default. The server asks for it only
			// after failed attempts, and then we fetch and display it.
			if a.captchaSVG != "" {
				if svg, err := ui.ParseSVG([]byte(a.captchaSVG)); err == nil {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Image(c, svg).Width(160).Height(50).Radius(9).
							Border(1, t.Border.Alpha(0.4))
						if ghost(c, "换一张", 36).Clicked() {
							a.refreshCaptcha()
						}
					})
				}
				field(c, "验证码", &a.code, "请输入图中字符", false)
			}

			if a.loginError != "" {
				ui.Box(c).Padding(10, 12).Radius(9).
					Material(glass.Glass{Tint: ui.Hex("#e5484d").Alpha(0.28)}).Children(func() {
					ui.Text(c, a.loginError).FontSize(12).TextColor(ui.Hex("#ff9095"))
				})
			}
			loginLabel := "登录"
			if a.busy {
				loginLabel = "登录中…"
			}
			if primary(c, loginLabel, 0).FillWidth().Disabled(a.busy).Clicked() {
				a.doLogin()
			}
			ui.Text(c, "由 Vercel · Supabase · MyGo 驱动").FontSize(11).
				TextColor(t.TextMuted).Justify(ui.Center)
		})
	})
}

func (a *app) filesContent(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Grow(1).Padding(10, 16, 16, 12).Gap(14).Children(func() {
		// Sidebar: brand, navigation, folder creation, account at the bottom.
		softPanel(c).Width(224).FillHeight().Padding(18, 14).Gap(7).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 4, 10).Children(func() {
				brandMark(c, 34)
				ui.Column(c).Children(func() {
					ui.Text(c, "资源管家").FontSize(16).Bold()
					ui.Text(c, "文件与分享").FontSize(10).TextColor(t.TextMuted)
				})
			})
			divider(c)
			ui.Box(c).Height(8)
			ui.Text(c, "工作区").FontSize(11).TextColor(t.TextMuted).Padding(0, 6)
			if iconButton(c, iconFolder, "全部文件", 38, false).FillWidth().Clicked() {
				a.current = ""
				a.crumbs = nil
				a.reload()
			}
			ui.Box(c).Grow(1)
			divider(c)
			ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(6, 4).Children(func() {
				initial := "?"
				if len(a.email) > 0 {
					initial = strings.ToUpper(a.email[:1])
				}
				ui.Box(c).Width(30).Height(30).Radius(15).
					Background(t.Accent.Alpha(0.75)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, initial).FontSize(13).Bold().TextColor(t.AccentText)
				})
				ui.Text(c, a.email).FontSize(11).TextColor(t.TextMuted).Grow(1)
			})
			if iconButton(c, iconSettings, "设置", 32, false).FillWidth().Clicked() {
				a.dirDraft = a.settings.DownloadDir
				if a.dirDraft == "" {
					a.dirDraft = a.defaultDownloadDir()
				}
				a.limitDraft = strconv.Itoa(a.maxDownloads())
				a.settingsOpen = true
				a.loadAllFolders()
			}
			if iconButton(c, iconLogout, "退出登录", 32, false).FillWidth().Clicked() {
				a.confirmLogout = true
			}
		})

		// Main column: toolbar card + file card. FillHeight is essential here:
		// a Row centers its children vertically, so without it the whole
		// content block drops into the middle of the window.
		ui.Column(c).Grow(1).FillHeight().Gap(14).Children(func() {
			if a.notice != "" {
				ttl := 6 * time.Second
				if a.noticeKind == "success" {
					ttl = 4 * time.Second
				}
				if time.Since(a.noticeAt) > ttl {
					a.notice, a.noticeKind = "", ""
				} else {
					// Keep asking for frames so the toast can expire on time.
					c.After(300 * time.Millisecond)
					tint := ui.Transparent
					textColor := t.Accent
					switch a.noticeKind {
					case "error":
						tint, textColor = ui.Hex("#e5484d").Alpha(0.28), ui.Hex("#ff9095")
					case "success":
						tint, textColor = ui.Hex("#30a46c").Alpha(0.26), ui.Hex("#63dba1")
					}
					ui.Row(c).Padding(9, 12).Gap(10).AlignItems(ui.Center).Radius(9).
						Material(glass.Glass{Tint: tint}).Children(func() {
						ui.Text(c, a.notice).FontSize(12).TextColor(textColor).Grow(1)
						if iconOnlyButton(c, iconX, 24).Clicked() {
							a.notice, a.noticeKind = "", ""
						}
					})
				}
			}
			bar := panel(c)
			bar.Padding(14, 18).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					if iconOnlyButton(c, iconChevronLeft, 34).Disabled(a.current == "").Clicked() {
						a.goBack()
					}
					ui.Row(c).Gap(4).AlignItems(ui.Center).Grow(1).Children(func() {
						if ghost(c, "全部文件", 28).Clicked() {
							a.openFolderID("")
						}
						for _, crumb := range a.crumbs {
							ui.Text(c, "›").FontSize(12).TextColor(t.TextMuted)
							part := crumb
							if ghost(c, part.Name, 28).Clicked() {
								a.openFolderID(part.ID)
							}
						}
					})
					ui.Text(c, fmt.Sprintf("共 %d 个文件", len(a.files))).
						FontSize(11).TextColor(t.TextMuted)
				})
				ui.Box(c).Height(8)
				// The tool cluster is one Liquid Glass surface, as macOS draws
				// a toolbar: panes fuse while they nearly touch.
				modeIcon := iconGrid
				if a.viewMode == "grid" {
					modeIcon = iconList
				}
				tools := glass.NewGroup(c)
				tools.Radius = 9
				tools.Merge = 0
				tools.Gap(8).Children(func() {
					search := ui.TextInput(c, &a.query).Placeholder("搜索文件…").
						Width(220).Height(34).Radius(9).Background(ui.Transparent)
					if search.Submitted() {
						a.reload()
					}
					tools.Pane(search)
					if tools.Pane(iconButtonBare(c, iconSearch, "搜索", 34, false)).Clicked() {
						a.reload()
					}
					if tools.Pane(iconButtonBare(c, iconRefresh, "刷新", 34, false)).Clicked() {
						a.reload()
					}
					center := ui.Row(c).Gap(5).Height(34).Padding(0, 10).AlignItems(ui.Center).Radius(9)
					center.Children(func() {
						ui.Icon(c, iconUpload).FontSize(17).TextColor(t.Accent)
						if len(a.uploads) > 0 {
							ui.Text(c, fmt.Sprintf("%d", len(a.uploads))).FontSize(11).TextColor(t.Accent)
						}
					})
					if tools.Pane(center).Clicked() {
						a.uploadCenter = !a.uploadCenter
					}
					if tools.Pane(iconOnlyButtonBare(c, modeIcon, 34)).Clicked() {
						if a.viewMode == "grid" {
							a.viewMode = "list"
						} else {
							a.viewMode = "grid"
						}
					}
					if tools.Pane(iconOnlyButtonBare(c, iconSort, 34)).Clicked() {
						a.sortAsc = !a.sortAsc
						a.reload()
					}

					// The primary action is the last pane of the same group.
					uploadPane := ui.Row(c).Height(34).Padding(0, 11).Gap(7).
						AlignItems(ui.Center).Justify(ui.Center).Radius(9).Background(t.Accent)
					uploadPane.Children(func() {
						ui.Icon(c, iconUpload).FontSize(15).TextColor(t.AccentText)
						ui.Text(c, "上传文件").FontSize(12).TextColor(t.AccentText)
					})
					if uploadPane.Clicked() {
						a.menuOpen, a.menuID = true, "upload"
					}
					tools.Pane(uploadPane)
					a.glassPopover(c, uploadPane, "upload", a.uploadMenuItems())
				})
			})

			listCard := panel(c)
			listCard.Grow(1).Padding(16, 18).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "文件夹").FontSize(12).TextColor(t.TextMuted).Grow(1)
						if iconOnlyButton(c, iconPlus, 26).Clicked() {
							a.folderNameDraft, a.createFolderOpen = "", true
						}
					})
					if len(a.folders) > 0 && a.viewMode == "grid" {
						a.folderGrid.Key = func(i int) any { return a.folders[i].ID }
						ui.GridView(c, &a.folderGrid, len(a.folders), 150, 88, func(i int) {
							folder := a.folders[i]
							card := ui.Box(c).Key(folder.ID).Padding(10).Gap(4).
								Radius(12).Material(glass.Glass{})
							card.Children(func() {
								ui.Icon(c, iconFolderOpen).FontSize(20).TextColor(t.Accent)
								ui.Text(c, folder.Name).FontSize(12).MaxLines(1).Ellipsis("…")
								ui.Text(c, fmt.Sprintf("%d 个文件", folder.FileCount)).
									FontSize(10).TextColor(t.TextMuted)
							})
							if card.Clicked() {
								a.openFolder(folder)
							}
						}).MaxHeight(106).Gap(10)
						divider(c)
					}
					if a.viewMode == "grid" {
						ui.Row(c).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "文件").FontSize(12).TextColor(t.TextMuted)
						})
					}
					if a.loadError != "" {
						ui.Text(c, a.loadError).FontSize(12).TextColor(ui.Hex("#ff6369"))
					}
					if a.loading {
						ui.Column(c).Fill().Center().Gap(8).Children(func() {
							ui.Spinner(c)
							ui.Text(c, "加载中…").FontSize(12).TextColor(t.TextMuted)
						})
					} else if len(a.files) > 0 {
						if a.viewMode == "grid" {
							a.grid.Key = func(i int) any { return a.files[i].ID }
							a.grid.Label = func(i int) string { return a.files[i].Name }
							minWidth, cardHeight := a.gridMetrics()
							previewHeight := cardHeight - 64
							if previewHeight < 52 {
								previewHeight = 52
							}
							ui.GridView(c, &a.grid, len(a.files), minWidth, cardHeight, func(i int) {
								f := a.files[i]
								kind := fileKind(f.Name, f.MimeType)
								card := ui.Box(c).Key(f.ID).Grow(1).Padding(12).Gap(8).
									Radius(14).Material(glass.Glass{}).Perspective(700)
								// 3D tilt: the side under the pointer is pressed
								// inward; angles ease back to zero on leave.
								tiltX, tiltY, lift := float32(0), float32(0), float32(0)
								if card.Hovered() {
									card.Border(1, t.Accent.Alpha(0.45))
									if x, y, inside := card.PointerPosition(); inside {
										nx := (x - minWidth/2) / (minWidth / 2)
										ny := (y - cardHeight/2) / (cardHeight / 2)
										if nx < -1 {
											nx = -1
										} else if nx > 1 {
											nx = 1
										}
										if ny < -1 {
											ny = -1
										} else if ny > 1 {
											ny = 1
										}
										tiltX, tiltY, lift = -ny*9, nx*9, 10
									}
								}
								card.RotateX(card.Animate("tilt-x", tiltX, 140*time.Millisecond)).
									RotateY(card.Animate("tilt-y", tiltY, 140*time.Millisecond)).
									TranslateZ(card.Animate("tilt-z", lift, 140*time.Millisecond))
								card.Children(func() {
									ui.Box(c).FillWidth().Height(previewHeight).Radius(10).
										Background(kind.Color.Alpha(0.12)).
										AlignItems(ui.Center).Justify(ui.Center).Children(func() {
										ui.Icon(c, kind.Icon).FontSize(34).TextColor(kind.Color)
									})
									ui.Column(c).Gap(3).Children(func() {
										ui.Text(c, f.Name).FontSize(13).MaxLines(1).Ellipsis("…")
										ui.Text(c, fmt.Sprintf("%s  ·  %s  ·  %s", humanSize(f.Size), kind.Label, uploadTime(f.CreatedAt))).
											FontSize(10).TextColor(t.TextMuted).MaxLines(1)
									})
									if a.downloads[f.ID] != nil {
										ui.Row(c).Absolute().Bottom(58).Right(24).Padding(6, 8).Gap(6).
											AlignItems(ui.Center).Radius(10).Material(glass.Glass{}).
											Children(func() {
												a.downloadIndicator(c, f.ID)
											})
									}
									var more ui.Element
									ui.Row(c).Absolute().Top(14).Right(16).Children(func() {
										more = iconOnlyButtonBare(c, iconMore, 26)
										if more.Clicked() {
											a.menuOpen, a.menuID = true, f.ID
										}
									})
									a.glassPopover(c, more, f.ID, a.fileMenuItems(f))
								})
							}).Grow(1)
						} else {
							// Finder/网盘 style: folders and files in one table.
							ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(0, 6).Children(func() {
								ui.Text(c, "文件名").FontSize(11).TextColor(t.TextMuted).Grow(1)
								ui.Text(c, "大小").FontSize(11).TextColor(t.TextMuted).Width(90).TextAlign(ui.End)
								ui.Box(c).Width(16)
								ui.Text(c, "文件类型").FontSize(11).TextColor(t.TextMuted).Width(90)
								ui.Text(c, "修改时间").FontSize(11).TextColor(t.TextMuted).Width(130)
								ui.Box(c).Width(32)
							})
							divider(c)
							total := len(a.folders) + len(a.files)
							a.list.Key = func(i int) any {
								if i < len(a.folders) {
									return "dir-" + a.folders[i].ID
								}
								return a.files[i-len(a.folders)].ID
							}
							ui.List(c, &a.list, total, func(i int) {
								if i < len(a.folders) {
									folder := a.folders[i]
									ui.Row(c).Key("dir-"+folder.ID).Height(46).Padding(0, 6).Gap(10).
										AlignItems(ui.Center).Children(func() {
										ui.Row(c).Gap(10).AlignItems(ui.Center).Grow(1).Children(func() {
											ui.Box(c).Width(32).Height(32).Radius(8).
												Background(t.Accent.Alpha(0.12)).
												AlignItems(ui.Center).Justify(ui.Center).Children(func() {
												ui.Icon(c, iconFolderOpen).FontSize(17).TextColor(t.Accent)
											})
											if ui.Text(c, folder.Name).FontSize(13).Grow(1).MaxLines(1).Ellipsis("…").Clicked() {
												a.openFolder(folder)
											}
										})
										ui.Text(c, "-").FontSize(11).TextColor(t.TextMuted).Width(90).TextAlign(ui.End)
										ui.Box(c).Width(16)
										ui.Text(c, "文件夹").FontSize(11).TextColor(t.TextMuted).Width(90)
										ui.Text(c, uploadTime(folder.CreatedAt)).FontSize(11).
											TextColor(t.TextMuted).Width(130)
										ui.Box(c).Width(32)
									})
									return
								}
								f := a.files[i-len(a.folders)]
								ui.Row(c).Key(f.ID).Height(46).Padding(0, 6).Gap(10).AlignItems(ui.Center).Children(func() {
									kind := fileKind(f.Name, f.MimeType)
									ui.Row(c).Gap(10).AlignItems(ui.Center).Grow(1).Children(func() {
										ui.Box(c).Width(32).Height(32).Radius(8).
											Background(kind.Color.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
											ui.Icon(c, kind.Icon).FontSize(17).TextColor(kind.Color)
										})
										if ui.Text(c, f.Name).FontSize(13).Grow(1).MaxLines(1).Ellipsis("…").Clicked() {
											a.openPreview(f)
										}
									})
									ui.Text(c, humanSize(f.Size)).FontSize(11).TextColor(t.TextMuted).Width(90).TextAlign(ui.End)
									ui.Box(c).Width(16)
									ui.Text(c, kind.Label).FontSize(11).TextColor(t.TextMuted).Width(90)
									ui.Text(c, uploadTime(f.CreatedAt)).FontSize(11).TextColor(t.TextMuted).Width(130)
									// One menu keeps the row compact, as in Finder.
									a.downloadIndicator(c, f.ID)
									more := iconOnlyButton(c, iconMore, 32)
									if more.Clicked() {
										a.menuOpen, a.menuID = true, f.ID
									}
									a.glassPopover(c, more, f.ID, a.fileMenuItems(f))
								})
							}).Grow(1)
						}
					} else {
						ui.Column(c).Fill().Center().Gap(6).Children(func() {
							ui.Icon(c, iconFolderOpen).FontSize(34).TextColor(t.TextMuted)
							ui.Text(c, "这里还没有文件").FontSize(14)
							ui.Text(c, "上传文件或新建文件夹开始使用").FontSize(12).TextColor(t.TextMuted)
						})
					}
					if a.shareURL != "" {
						ui.Text(c, "分享链接: "+a.shareURL).FontSize(12).TextColor(t.Accent)
					}
				}).Grow(1)
			})
		})
		a.uploadDrawer(c)
	})
}

func (a *app) uploadDrawer(c *ui.Context) {
	t := c.Theme()
	const drawerWidth float32 = 390

	// Dim the list behind the drawer; clicking the scrim closes it.
	if a.uploadCenter {
		scrim := ui.Box(c.Key("drawer-scrim")).Absolute().Top(0).Left(0).Right(0).Bottom(0).
			ZIndex(10).Background(ui.RGBA(4, 8, 20, 0.32))
		if scrim.Clicked() {
			a.uploadCenter = false
		}
	}

	target := float32(0)
	if a.uploadCenter {
		target = 1
	}
	drawer := ui.Box(c.Key("upload-drawer")).Absolute().Top(14).Right(14).Bottom(14).
		Width(drawerWidth).ZIndex(11).Radius(20)
	slide := drawer.Animate("slide", target, 220*time.Millisecond)
	drawer.Translate((1-slide)*(drawerWidth+14), 0).Padding(18, 18).
		Material(glass.Glass{}).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "上传中心").FontSize(17).Bold().Grow(1)
				if iconOnlyButton(c, iconX, 28).Clicked() {
					a.uploadCenter = false
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("任务  ·  %d", len(a.uploads))).FontSize(12).
					TextColor(t.TextMuted).Grow(1)
				if ghost(c, "清空已完成", 28).Clicked() {
					keep := a.uploads[:0]
					for _, task := range a.uploads {
						if task.Status == "queued" || task.Status == "uploading" {
							keep = append(keep, task)
						}
					}
					a.uploads = keep
				}
			})

			if len(a.uploads) == 0 {
				ui.Column(c).Fill().Center().Gap(6).Children(func() {
					ui.Icon(c, iconUpload).FontSize(30).TextColor(t.TextMuted)
					ui.Text(c, "还没有上传任务").FontSize(14)
					ui.Text(c, "选择类型后点击「上传文件」开始").FontSize(12).TextColor(t.TextMuted)
				})
			} else {
				a.uploadList.Key = func(i int) any { return a.uploads[i].ID }
				ui.List(c, &a.uploadList, len(a.uploads), func(i int) {
					task := a.uploads[i]
					kind := fileKind(task.Name, "")
					status := "等待中"
					statusColor := t.TextMuted
					switch task.Status {
					case "uploading":
						status, statusColor = fmt.Sprintf("上传中 %d%%", task.Percent), t.Accent
					case "done":
						status, statusColor = "已完成", ui.Hex("#63dba1")
					case "error":
						status, statusColor = "失败", ui.Hex("#ff9095")
					}
					id := task.ID
					ui.Row(c).Key(id).Height(64).Padding(0, 6).Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Width(36).Height(36).Radius(9).
							Background(kind.Color.Alpha(0.14)).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
							ui.Icon(c, kind.Icon).FontSize(19).TextColor(kind.Color)
						})
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, task.Name).FontSize(13).Grow(1)
								ui.Text(c, status).FontSize(11).TextColor(statusColor)
							})
							if task.Status == "uploading" || task.Status == "queued" {
								ui.Progress(c, float64(task.Percent)/100).FillWidth().TextColor(t.Accent)
							} else if task.Err != "" {
								ui.Text(c, task.Err).FontSize(10).TextColor(ui.Hex("#ff9095"))
							} else {
								ui.Text(c, humanSize(task.Size)).FontSize(10).TextColor(t.TextMuted)
							}
						})
						if task.Status == "done" || task.Status == "error" {
							if iconOnlyButton(c, iconX, 24).Clicked() {
								keep := a.uploads[:0]
								for _, u := range a.uploads {
									if u.ID != id {
										keep = append(keep, u)
									}
								}
								a.uploads = keep
							}
						}
					})
				}).Grow(1)
			}
		}).Grow(1)
	})
}

func uploadTime(raw string) string {
	if raw == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	if len(raw) >= 16 {
		return strings.Replace(raw[:16], "T", " ", 1)
	}
	return ""
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

func openExternal(raw string) {
	if raw == "" {
		return
	}
	_ = mygo.Shell.OpenExternal(raw)
}

// ---------- actions ----------

func (a *app) refreshCaptcha() {
	a.async(func() {
		id, svg, err := a.api.Captcha()
		a.win.Update(func() {
			if err != nil {
				a.loginError = "无法获取验证码: " + err.Error()
				return
			}
			a.captchaID, a.captchaSVG, a.code = id, svg, ""
		})
	})
}

func (a *app) doLogin() {
	a.busy = true
	a.loginError = ""
	email, pass := a.email, a.pass
	id, code := a.captchaID, a.code
	a.async(func() {
		out, err := a.api.Login(email, pass, id, code)
		a.win.Update(func() {
			a.busy = false
			if err != nil {
				a.loginError = err.Error()
				return
			}
			if !out.OK {
				a.loginError = out.Message
				if out.CaptchaRequired {
					a.refreshCaptcha()
				}
				return
			}
			a.screen = "files"
			a.email = email
			a.captchaID, a.captchaSVG, a.code = "", "", ""
			a.loadAllFolders()
			if err := a.api.SaveSession(a.sessionFile); err != nil {
				a.setError("会话保存失败: " + err.Error())
			}
			a.reload()
		})
	})
}

func (a *app) reload() {
	folder, query := a.current, a.query
	a.loading = true
	a.loadError = ""
	a.async(func() {
		files, ferr := a.api.Files(folder, query, 1, 100, a.sortAsc)
		folders, derr := a.api.Folders(folder)
		a.win.Update(func() {
			a.loading = false
			if ferr != nil {
				a.loadError = "加载文件失败: " + ferr.Error()
			} else {
				a.files = files.Files
			}
			if derr == nil {
				a.folders = folders
			}
		})
	})
}

func (a *app) folderPath(id string) []api.Folder {
	byID := map[string]api.Folder{}
	for _, f := range a.allFolders {
		byID[f.ID] = f
	}
	var path []api.Folder
	seen := map[string]bool{}
	for cur := id; cur != "" && !seen[cur]; {
		f, ok := byID[cur]
		if !ok {
			break
		}
		seen[cur] = true
		path = append([]api.Folder{f}, path...)
		cur = f.ParentID
	}
	return path
}

func (a *app) openFolderID(id string) {
	a.current = id
	a.crumbs = a.folderPath(id)
	a.query = ""
	a.reload()
}

func (a *app) openFolder(f api.Folder) {
	a.openFolderID(f.ID)
}

// folderTree renders the sidebar hierarchy recursively from the flat list.
func (a *app) folderTree(c *ui.Context, parentID string, depth int) {
	t := c.Theme()
	for _, f := range a.allFolders {
		if f.ParentID != parentID {
			continue
		}
		folder := f
		hasChildren := false
		for _, child := range a.allFolders {
			if child.ParentID == folder.ID {
				hasChildren = true
				break
			}
		}
		ui.Row(c).Key(folder.ID).Gap(4).AlignItems(ui.Center).
			Padding(0, 4, 0, 4+float32(depth)*12).Children(func() {
			if hasChildren {
				icon := iconChevronRight
				if a.expanded[folder.ID] {
					icon = iconChevronDown
				}
				if ui.Icon(c, icon).FontSize(12).TextColor(t.TextMuted).Clicked() {
					a.expanded[folder.ID] = !a.expanded[folder.ID]
				}
			} else {
				ui.Box(c).Width(12)
			}
			if ui.Text(c, folder.Name).FontSize(13).Grow(1).MaxLines(1).Ellipsis("…").Clicked() {
				a.openFolderID(folder.ID)
			}
		})
		if a.expanded[folder.ID] {
			a.folderTree(c, folder.ID, depth+1)
		}
	}
}

func (a *app) goBack() {
	if len(a.crumbs) == 0 {
		return
	}
	a.crumbs = a.crumbs[:len(a.crumbs)-1]
	if len(a.crumbs) == 0 {
		a.current = ""
	} else {
		a.current = a.crumbs[len(a.crumbs)-1].ID
	}
	a.reload()
}

func (a *app) createFolder(name string) {
	a.async(func() {
		err := a.api.CreateFolder(name, a.current)
		a.win.Update(func() {
			if err != nil {
				a.setError("新建文件夹失败: " + err.Error())
			} else {
				a.reload()
			}
		})
	})
}

type menuEntry struct {
	Label  string
	Run    func()
	Danger bool
}

// glassPopover draws an in-window menu panel with Liquid Glass. The system
// menu cannot use materials, so both the upload-type menu and row actions use
// this instead.
func (a *app) glassPopover(c *ui.Context, anchor ui.Element, id string, items []menuEntry) {
	if !a.menuOpen || a.menuID != id {
		return
	}
	ui.PopoverBase(c, anchor, &a.menuOpen, func(panel ui.Element) {
		// PopoverBase has no theme background: the glass material is applied
		// to the panel itself, so no dark sheet shows through the translucency.
		panel.Padding(6).Gap(2).Radius(12).NoShadow().Material(glass.Glass{})
		panel.Children(func() {
			for _, item := range items {
				entry := item
				textColor := c.Theme().Text
				if entry.Danger {
					textColor = ui.Hex("#ff9095")
				}
				button := ui.Button(c, entry.Label).FillWidth().Height(30).Radius(8).
					Background(ui.Transparent).Border(0, ui.Transparent).TextColor(textColor)
				// On hover the row becomes a small glass bubble: a tinted,
				// rounded pane floating above the menu's glass surface.
				if button.Hovered() || button.Pressed() {
					button.Radius(10).Material(glass.Glass{
						Tint: c.Theme().Accent.Alpha(0.22), Interactive: true,
					})
				}
				if button.Clicked() {
					entry.Run()
					a.menuOpen, a.menuID = false, ""
				}
			}
		})
	})
}

func loadSettings(path string) appSettings {
	var s appSettings
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	if s.MaxDownloads < 1 || s.MaxDownloads > 10 {
		s.MaxDownloads = 1
	}
	return s
}

func (a *app) maxDownloads() int {
	n := a.settings.MaxDownloads
	if n < 1 || n > 10 {
		return 1
	}
	return n
}

func saveSettings(path string, s appSettings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (a *app) defaultDownloadDir() string {
	if dir, err := mygo.App.Path(mygo.PathDownloads); err == nil && dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Downloads")
}

func safeFileName(name string) string {
	return strings.NewReplacer(
		`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_",
		`"`, "_", "<", "_", ">", "_", "|", "_",
	).Replace(name)
}

func (a *app) downloadFile(f api.File) {
	if task, ok := a.downloads[f.ID]; ok && (task.Status == "downloading" || task.Status == "queued") {
		return
	}
	task := &downloadTask{ID: f.ID, URL: f.URL, Name: f.Name, Status: "queued"}
	a.downloads[f.ID] = task
	a.downloadQueue = append(a.downloadQueue, task)
	a.pumpDownloads()
}

// pumpDownloads starts queued tasks up to the configured concurrency limit.
func (a *app) pumpDownloads() {
	for a.activeDownloads < a.maxDownloads() && len(a.downloadQueue) > 0 {
		task := a.downloadQueue[0]
		a.downloadQueue = a.downloadQueue[1:]
		a.activeDownloads++
		task.Status = "downloading"
		a.startDownload(task)
	}
}

func (a *app) startDownload(task *downloadTask) {
	a.async(func() {
		dir := strings.TrimSpace(a.settings.DownloadDir)
		if dir == "" {
			dir = a.defaultDownloadDir()
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			a.win.Update(func() {
				a.activeDownloads--
				task.Status, task.Err = "error", err.Error()
				a.setError("下载失败: " + err.Error())
				a.pumpDownloads()
			})
			return
		}
		dest := filepath.Join(dir, safeFileName(task.Name))
		base, ext := strings.TrimSuffix(dest, filepath.Ext(dest)), filepath.Ext(dest)
		for n := 1; ; n++ {
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				break
			}
			dest = fmt.Sprintf("%s-%d%s", base, n, ext)
		}
		task.Path = dest
		err := api.DownloadToFile(task.URL, dest, func(done, total int64) {
			pct := 0
			if total > 0 {
				pct = int(float64(done) * 100 / float64(total))
			}
			a.win.Update(func() {
				task.Done, task.Total, task.Percent = done, total, pct
			})
		})
		a.win.Update(func() {
			a.activeDownloads--
			if err != nil {
				task.Status, task.Err = "error", err.Error()
				a.setError("下载失败: " + err.Error())
			} else {
				task.Status, task.Percent = "done", 100
				a.setSuccess("已下载到: " + dest)
			}
			a.pumpDownloads()
		})
	})
}

func (a *app) logout() {
	a.api.ClearSession(a.sessionFile)
	a.setAPI()
	a.screen = "login"
	a.files, a.folders, a.email, a.notice = nil, nil, "", ""
	a.noticeKind, a.uploading, a.uploadPct = "", false, 0
	a.confirmLogout = false
	a.closePreview()
}

func (a *app) closePreview() {
	a.preview, a.previewImage, a.previewErr, a.previewStage = nil, nil, "", ""
	a.previewLoading = false
}

func (a *app) openPreview(f api.File) {
	mime := strings.ToLower(f.MimeType)
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.Name), "."))
	isImage := strings.HasPrefix(mime, "image/")
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg":
		isImage = true
	}
	if isImage {
		if f.URL == "" {
			a.setError("该文件没有可预览的地址")
			return
		}
		a.preview, a.previewImage, a.previewErr = &f, nil, ""
		a.previewLoading, a.previewStage = true, "加载中…"
		a.async(func() {
			data, err := api.FetchAsset(f.URL)
			// Decode on the main thread: MyGo keeps bitmaps on the GPU and
			// forwards off-thread calls, which can stall on large images.
			a.win.Update(func() {
				if err == nil {
					a.previewImage, err = ui.DecodeImage(data)
				}
				a.previewLoading, a.previewStage = false, ""
				if err != nil {
					a.previewErr = "预览加载失败: " + err.Error()
				}
			})
		})
		return
	}
	// PDFs, video, audio and plain text render in an in-app WebView window.
	if mime == "application/pdf" || strings.HasPrefix(mime, "video/") ||
		strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "text/") {
		mygo.NewWindow(mygo.WindowOptions{
			Title: f.Name, URL: f.URL, Width: 1000, Height: 700, StateKey: "preview",
		})
		return
	}
	a.setError("该格式不支持应用内预览，请下载后查看")
}

func (a *app) fileMenuItems(f api.File) []menuEntry {
	file := f
	return []menuEntry{
		{Label: "预览", Run: func() { a.openPreview(file) }},
		{Label: "下载", Run: func() { a.downloadFile(file) }},
		{Label: "分享", Run: func() { a.makeShare(file.ID) }},
		{Label: "删除", Run: func() { a.deleteFile(file.ID) }, Danger: true},
	}
}

func (a *app) uploadMenuItems() []menuEntry {
	items := make([]menuEntry, 0, len(uploadCategories))
	for _, category := range uploadCategories {
		cat := category
		label := cat.Label
		if cat.Key == "" {
			label = "全部文件"
		}
		items = append(items, menuEntry{Label: label, Run: func() {
			a.uploadFilter = cat.Key
			a.pickAndUpload()
		}})
	}
	return items
}

func (a *app) makeShare(fileID string) {
	a.async(func() {
		u, err := a.api.Share(fileID, 0)
		a.win.Update(func() {
			if err != nil {
				a.setError("分享失败: " + err.Error())
			} else {
				a.shareURL = u
				_ = mygo.Shell.OpenExternal(u)
			}
		})
	})
}

func (a *app) deleteFile(id string) {
	a.async(func() {
		err := a.api.DeleteFile(id)
		a.win.Update(func() {
			if err != nil {
				a.setError("删除失败: " + err.Error())
			} else {
				a.reload()
			}
		})
	})
}

func (a *app) pickAndUpload() {
	cat := uploadCategories[0]
	for _, c := range uploadCategories {
		if c.Key == a.uploadFilter {
			cat = c
			break
		}
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:    "选择要上传的文件",
		Multiple: true,
		Parent:   a.win,
		Filters: []mygo.FileFilter{{
			Name: cat.Label, Extensions: cat.Extensions, MIMETypes: cat.MIMETypes,
		}},
	})
	if err != nil || len(paths) == 0 {
		return
	}

	tasks := make([]*uploadTask, 0, len(paths))
	for _, path := range paths {
		name := filepath.Base(path)
		size := int64(0)
		if info, statErr := os.Stat(path); statErr == nil {
			size = info.Size()
		}
		tasks = append(tasks, &uploadTask{
			ID:   fmt.Sprintf("%d-%s", time.Now().UnixNano(), name),
			Path: path, Name: name, Size: size, Status: "queued",
		})
	}

	// State changes happen on the main thread; network work runs on workers.
	a.uploadCenter = true
	a.uploads = append(a.uploads, tasks...)
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(t *uploadTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			a.runUpload(t)
		}(task)
	}
	go func() {
		wg.Wait()
		a.win.Update(func() {
			done, failed := 0, 0
			for _, t := range a.uploads {
				if t.Status == "done" {
					done++
				} else if t.Status == "error" {
					failed++
				}
			}
			if failed > 0 {
				a.setError(fmt.Sprintf("上传结束：%d 成功，%d 失败", done, failed))
			} else if done > 0 {
				a.setSuccess(fmt.Sprintf("已上传 %d 个文件", done))
			}
			a.reload()
		})
	}()
}

func (a *app) failUpload(t *uploadTask, err error) {
	msg := err.Error()
	a.win.Update(func() {
		t.Status, t.Err = "error", msg
	})
}

func (a *app) runUpload(t *uploadTask) {
	a.win.Update(func() { t.Status, t.Percent = "uploading", 5 })
	data, err := os.ReadFile(t.Path)
	if err != nil {
		a.failUpload(t, err)
		return
	}
	// The server chooses the key and the upload host (QINIU_ZONE).
	ticket, err := a.api.UploadTicket(t.Name, a.current)
	if err != nil {
		a.failUpload(t, err)
		return
	}
	a.win.Update(func() { t.Percent = 10 })
	err = a.api.UploadToQiniu(ticket.UploadURL, ticket.Token, ticket.Key, t.Name, data,
		func(pct int) {
			p := 10 + pct*80/100
			a.win.Update(func() { t.Percent = p })
		})
	if err != nil {
		a.failUpload(t, err)
		return
	}
	a.win.Update(func() { t.Percent = 92 })
	err = a.api.CreateFile(t.Name, ticket.Key, int64(len(data)), mimeType(t.Name), a.current)
	if err != nil {
		a.failUpload(t, err)
		return
	}
	a.win.Update(func() { t.Status, t.Percent = "done", 100 })
}

func mimeType(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}

// loadEnv reads a local, git-ignored .env next to the working directory or
// the executable. Real environment variables take precedence over file values.
func loadEnv() {
	paths := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), ".env"))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), `"'`)
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func main() {
	loadEnv()
	a := &app{
		base: defaultBaseURL, viewMode: "list",
		downloads: map[string]*downloadTask{}, expanded: map[string]bool{},
	}
	a.setAPI()
	if env := strings.TrimSpace(os.Getenv("RESOURCE_HUB_API")); env != "" {
		a.base = env
		a.setAPI()
	}
	configDir, _ := os.UserConfigDir()
	configDir = filepath.Join(configDir, "resource-hub-desktop")
	a.sessionFile = filepath.Join(configDir, "session.json")
	a.settingsPath = filepath.Join(configDir, "settings.json")
	a.settings = loadSettings(a.settingsPath)
	_ = os.MkdirAll(configDir, 0700)
	if err := a.api.LoadSession(a.sessionFile); err == nil {
		a.restoring = true
	}
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "资源管家",
			Width:     1400,
			Height:    925,
			MinWidth:  900,
			MinHeight: 620,
			// Liquid Glass: translucent window over the desktop.
			Transparent:   true,
			Vibrancy:      mygo.VibrancyGlass,
			TitleBarStyle: mygo.TitleBarHidden,
			StateKey:      "main-1400",
			Content:       ui.View(a.view),
		})
		a.win = win
		// Restore a previous login if the saved session is still valid.
		go func() {
			email, ok := a.api.SessionEmail()
			win.Update(func() {
				a.restoring = false
				if ok {
					a.email = email
					a.screen = "files"
					a.loadAllFolders()
					a.reload()
				} else {
					a.screen = "login"
				}
			})
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
