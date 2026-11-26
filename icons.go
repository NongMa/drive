package main

import (
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// These paths come from lucide-react v0.417.0 (ISC), already used by the web
// frontend. Parsing once here gives MyGo native, GPU-drawn icons.
func lucide(paths string) *ui.SVG {
	data := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + paths + `</svg>`
	icon, err := ui.ParseSVG([]byte(data))
	if err != nil {
		panic(err)
	}
	return icon
}

const fileOutline = `<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>`

var (
	iconGeneric      = lucide(fileOutline)
	iconImage        = lucide(fileOutline + `<circle cx="10" cy="12" r="2"/><path d="m20 17-1.296-1.296a2.41 2.41 0 0 0-3.408 0L9 22"/>`)
	iconVideo        = lucide(fileOutline + `<path d="m10 11 5 3-5 3v-6Z"/>`)
	iconAudio        = lucide(`<path d="M17.5 22h.5a2 2 0 0 0 2-2V7l-5-5H6a2 2 0 0 0-2 2v3"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M2 19a2 2 0 1 1 4 0v1a2 2 0 1 1-4 0v-4a6 6 0 0 1 12 0v4a2 2 0 1 1-4 0v-1a2 2 0 1 1 4 0"/>`)
	iconText         = lucide(fileOutline + `<path d="M10 9H8"/><path d="M16 13H8"/><path d="M16 17H8"/>`)
	iconSheet        = lucide(fileOutline + `<path d="M8 13h2"/><path d="M14 13h2"/><path d="M8 17h2"/><path d="M14 17h2"/>`)
	iconArchive      = lucide(`<path d="M10 12v-1"/><path d="M10 18v-2"/><path d="M10 7V6"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M15.5 22H18a2 2 0 0 0 2-2V7l-5-5H6a2 2 0 0 0-2 2v16a2 2 0 0 0 .274 1.01"/><circle cx="10" cy="20" r="2"/>`)
	iconCode         = lucide(`<path d="m10 12.5-2 2.5 2 2.5"/><path d="m14 12.5 2 2.5-2 2.5"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/>`)
	iconUpload       = lucide(`<path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"/><path d="M12 12v9"/><path d="m16 16-4-4-4 4"/>`)
	iconFolder       = lucide(`<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>`)
	iconFolderOpen   = lucide(`<path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"/>`)
	iconChevronLeft  = lucide(`<path d="m15 18-6-6 6-6"/>`)
	iconX            = lucide(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)
	iconMore         = lucide(`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`)
	iconSearch       = lucide(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)
	iconEye          = lucide(`<path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>`)
	iconDownload     = lucide(`<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" x2="12" y1="15" y2="3"/>`)
	iconShare        = lucide(`<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="m8.59 13.51 6.83 3.98"/><path d="m15.41 6.51-6.82 3.98"/>`)
	iconTrash        = lucide(`<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/>`)
	iconRefresh      = lucide(`<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>`)
	iconLogout       = lucide(`<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/>`)
	iconPlus         = lucide(`<path d="M5 12h14"/><path d="M12 5v14"/>`)
	iconGrid         = lucide(`<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/><rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>`)
	iconList         = lucide(`<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/><path d="M14 4h7"/><path d="M14 9h7"/><path d="M14 15h7"/><path d="M14 20h7"/>`)
	iconSort         = lucide(`<path d="m21 16-4 4-4-4"/><path d="M17 20V4"/><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/>`)
	iconSettings     = lucide(`<path d="M4 21v-7"/><path d="M4 10V3"/><path d="M12 21v-9"/><path d="M12 8V3"/><path d="M20 21v-5"/><path d="M20 12V3"/><path d="M1 14h6"/><path d="M9 8h6"/><path d="M17 16h6"/>`)
	iconChevronRight = lucide(`<path d="m9 18 6-6-6-6"/>`)
	iconChevronDown  = lucide(`<path d="m6 9 6 6 6-6"/>`)
)

type fileKindInfo struct {
	Icon  *ui.SVG
	Label string
	Color ui.Color
}

var (
	kindImage   = fileKindInfo{iconImage, "图片", ui.Hex("#3b82f6")}
	kindVideo   = fileKindInfo{iconVideo, "视频", ui.Hex("#a78bfa")}
	kindAudio   = fileKindInfo{iconAudio, "音频", ui.Hex("#f59e0b")}
	kindPDF     = fileKindInfo{iconText, "PDF", ui.Hex("#ef4444")}
	kindSheet   = fileKindInfo{iconSheet, "表格", ui.Hex("#10b981")}
	kindArchive = fileKindInfo{iconArchive, "压缩包", ui.Hex("#d97706")}
	kindCode    = fileKindInfo{iconCode, "代码", ui.Hex("#06b6d4")}
	kindDoc     = fileKindInfo{iconText, "文档", ui.Hex("#60a5fa")}
	kindOther   = fileKindInfo{iconGeneric, "文件", ui.Hex("#94a3b8")}
)

func fileKind(name, mime string) fileKindInfo {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "heic", "avif":
		return kindImage
	case "mp4", "mov", "avi", "mkv", "webm", "m4v":
		return kindVideo
	case "mp3", "wav", "flac", "aac", "m4a", "ogg", "wma":
		return kindAudio
	case "pdf":
		return kindPDF
	case "xls", "xlsx", "csv", "ods", "numbers":
		return kindSheet
	case "zip", "rar", "7z", "tar", "gz", "bz2":
		return kindArchive
	case "js", "ts", "tsx", "jsx", "go", "py", "java", "json", "html", "css", "sh", "sql", "yaml", "yml":
		return kindCode
	case "doc", "docx", "txt", "md", "rtf", "pages":
		return kindDoc
	}
	switch {
	case strings.HasPrefix(mime, "image/"):
		return kindImage
	case strings.HasPrefix(mime, "video/"):
		return kindVideo
	case strings.HasPrefix(mime, "audio/"):
		return kindAudio
	case strings.Contains(mime, "pdf"):
		return kindPDF
	case strings.Contains(mime, "spreadsheet") || strings.Contains(mime, "excel"):
		return kindSheet
	case strings.Contains(mime, "document") || strings.HasPrefix(mime, "text/"):
		return kindDoc
	}
	return kindOther
}

// typeLabel gives a short, human-readable format, e.g. "JPG" or "PDF".
func typeLabel(name, mime string) string {
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext != "" {
		return ext
	}
	if i := strings.Index(mime, "/"); i >= 0 {
		return strings.ToUpper(mime[i+1:])
	}
	return "FILE"
}
