package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const previewHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>TRMNL Mawaqit Preview</title>
<link rel="stylesheet" href="https://trmnl.com/css/latest/plugins.css">
<style>
  body {
    background: #1a1a1a;
    color: #ccc;
    font-family: sans-serif;
    padding: 32px;
    margin: 0;
  }
  h1 { color: #fff; margin-bottom: 8px; }
  .subtitle { color: #888; margin-bottom: 32px; }
  .variant { margin-bottom: 40px; }
  .variant h2 {
    color: #aaa;
    font-size: 16px;
    margin-bottom: 8px;
    font-weight: normal;
  }
  .frame {
    background: #fff;
    color: #000;
    overflow: hidden;
    border: 1px solid #333;
  }
</style>
</head>
<body class="environment trmnl">
<h1>Mawaqit Preview</h1>
<p class="subtitle">Mosque: {{.MosqueName}} &mdash; Timezone: {{.Timezone}}</p>

<div class="variant">
  <h2>Full (800 x 480)</h2>
  <div class="frame" style="width:800px;height:480px;">
    <div class="screen screen--2bit screen--md">
      <div class="view view--full">{{.Full}}</div>
    </div>
  </div>
</div>

<div class="variant">
  <h2>Half Horizontal (800 x 240)</h2>
  <div class="frame" style="width:800px;height:240px;">
    <div class="screen screen--2bit screen--md">
      <div class="view view--half_horizontal">{{.HalfHorizontal}}</div>
    </div>
  </div>
</div>

<div class="variant">
  <h2>Half Vertical (400 x 480)</h2>
  <div class="frame" style="width:400px;height:480px;">
    <div class="screen screen--2bit screen--md">
      <div class="view view--half_vertical">{{.HalfVertical}}</div>
    </div>
  </div>
</div>

<div class="variant">
  <h2>Quadrant (400 x 240)</h2>
  <div class="frame" style="width:400px;height:240px;">
    <div class="screen screen--2bit screen--md">
      <div class="view view--quadrant">{{.Quadrant}}</div>
    </div>
  </div>
</div>
</body>
</html>
`

type previewData struct {
	MosqueName     string
	Timezone       string
	Full           template.HTML
	HalfHorizontal template.HTML
	HalfVertical   template.HTML
	Quadrant       template.HTML
}

// runPreview handles the "preview" subcommand. It returns true if the
// subcommand was detected (so the caller should exit), or false if the
// program should continue with normal server startup.
func runPreview(args []string) bool {
	if len(args) < 2 || args[1] != "preview" {
		return false
	}

	fs := flag.NewFlagSet("preview", flag.ExitOnError)
	slug := fs.String("slug", "", "Mawaqit mosque slug (required)")
	timezone := fs.String("timezone", "UTC", "IANA timezone for prayer time computation")
	output := fs.String("output", "preview.html", "Output file path (use - for stdout)")
	fs.Parse(args[2:])

	if *slug == "" {
		fmt.Fprintln(os.Stderr, "error: --slug is required")
		fs.Usage()
		os.Exit(1)
	}

	loadEnvFile(".env")

	mawaqitBase := os.Getenv("MAWAQIT_API_BASE")
	if mawaqitBase == "" {
		log.Fatal("MAWAQIT_API_BASE environment variable is not set")
	}

	client := NewMawaqitClient(mawaqitBase)
	data, err := client.GetMosqueData(*slug, *timezone)
	if err != nil {
		log.Fatalf("fetch mosque data: %v", err)
	}

	pd, err := buildPrayerDisplay(data, *timezone)
	if err != nil {
		log.Fatalf("build prayer display: %v", err)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	}).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	result, err := renderAllMarkup(tmpl, pd)
	if err != nil {
		log.Fatalf("render markup: %v", err)
	}

	previewTmpl, err := template.New("preview").Parse(previewHTML)
	if err != nil {
		log.Fatalf("parse preview template: %v", err)
	}

	pdata := previewData{
		MosqueName:     pd.MosqueName,
		Timezone:       *timezone,
		Full:           template.HTML(result.Markup),
		HalfHorizontal: template.HTML(result.MarkupHalfHoriz),
		HalfVertical:   template.HTML(result.MarkupHalfVert),
		Quadrant:       template.HTML(result.MarkupQuadrant),
	}

	var w *os.File
	if *output == "-" {
		w = os.Stdout
	} else {
		w, err = os.Create(*output)
		if err != nil {
			log.Fatalf("create output file: %v", err)
		}
		defer w.Close()
	}

	var buf strings.Builder
	if err := previewTmpl.Execute(&buf, pdata); err != nil {
		log.Fatalf("execute preview template: %v", err)
	}

	if _, err := w.WriteString(buf.String()); err != nil {
		log.Fatalf("write output: %v", err)
	}

	if *output != "-" {
		absPath, _ := filepath.Abs(*output)
		fmt.Fprintf(os.Stderr, "Preview written to %s\n", absPath)
		openBrowser(absPath)
	}

	return true
}

func openBrowser(filePath string) {
	url := "file://" + filePath
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}
	cmd.Start()
}
