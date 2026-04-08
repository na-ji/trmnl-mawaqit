package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"time"
)

// templateToLiquid maps Go template names to their liquid output filenames.
var templateToLiquid = map[string]string{
	"full.html":            "full.liquid",
	"half_horizontal.html": "half_horizontal.liquid",
	"half_vertical.html":   "half_vertical.liquid",
	"quadrant.html":        "quadrant.liquid",
}

func mockPrayerDisplay() *PrayerDisplay {
	return &PrayerDisplay{
		MosqueName: "Mosquee Tawba",
		Prayers: []Prayer{
			{Name: "Fajr", Time: "05:42", IsNext: false},
			{Name: "Shuruq", Time: "07:14", IsNext: false},
			{Name: "Dohr", Time: "13:51", IsNext: false},
			{Name: "Asr", Time: "17:31", IsNext: false},
			{Name: "Maghrib", Time: "20:34", IsNext: true},
			{Name: "Isha", Time: "22:02", IsNext: false},
		},
		Jumua:  "12:30",
		Jumua2: "13:45",
	}
}

func generateLiquid(tmpl *template.Template, outDir string) error {
	pd := mockPrayerDisplay()

	for goName, liquidName := range templateToLiquid {
		html, err := renderTemplate(tmpl, goName, pd)
		if err != nil {
			return fmt.Errorf("render %s: %w", goName, err)
		}

		outPath := filepath.Join(outDir, liquidName)
		if err := os.WriteFile(outPath, []byte(html), 0644); err != nil {
			return fmt.Errorf("write %s: %w", outPath, err)
		}
		fmt.Printf("  %s -> %s\n", goName, outPath)
	}
	return nil
}

func templatesMaxModTime(dir string) time.Time {
	var maxMod time.Time
	entries, err := os.ReadDir(dir)
	if err != nil {
		return maxMod
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(maxMod) {
			maxMod = info.ModTime()
		}
	}
	return maxMod
}

func parseTemplates(dir string) (*template.Template, error) {
	return template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	}).ParseGlob(filepath.Join(dir, "*.html"))
}

// runLiquidGen handles the "liquidgen" subcommand. Returns true if it was invoked.
func runLiquidGen(args []string) bool {
	if len(args) < 2 || args[1] != "liquidgen" {
		return false
	}

	fs := flag.NewFlagSet("liquidgen", flag.ExitOnError)
	watch := fs.Bool("watch", false, "Watch templates/ for changes and rebuild automatically")
	outDir := fs.String("out", "src", "Output directory for .liquid files")
	templateDir := fs.String("templates", "templates", "Directory containing Go HTML templates")
	fs.Parse(args[2:])

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	build := func() {
		tmpl, err := parseTemplates(*templateDir)
		if err != nil {
			log.Printf("parse templates: %v", err)
			return
		}
		fmt.Println("Building liquid templates...")
		if err := generateLiquid(tmpl, *outDir); err != nil {
			log.Printf("generate: %v", err)
			return
		}
		fmt.Println("Done.")
	}

	build()

	if *watch {
		fmt.Printf("Watching %s/ for changes...\n", *templateDir)
		lastMod := templatesMaxModTime(*templateDir)
		for {
			time.Sleep(500 * time.Millisecond)
			current := templatesMaxModTime(*templateDir)
			if current.After(lastMod) {
				lastMod = current
				fmt.Println()
				build()
			}
		}
	}

	return true
}
