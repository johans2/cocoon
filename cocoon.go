package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const version = "0.1.0"

// --- Colors (256-color ANSI, same palette as larva) ---

const (
	colorReset  = "\033[0m"
	colorTeal   = "\033[38;5;37m"
	colorDim    = "\033[38;5;245m"
	colorBright = "\033[38;5;48m"
	colorErr    = "\033[38;5;208m"
	colorBold   = "\033[1m"
)

func teal(s string) string   { return colorTeal + s + colorReset }
func dim(s string) string    { return colorDim + s + colorReset }
func bright(s string) string { return colorBold + colorBright + s + colorReset }
func errclr(s string) string { return colorBold + colorErr + s + colorReset }

func printError(msg string, detail interface{}) {
	fmt.Fprintf(os.Stderr, "%s %v\n", errclr(msg), detail)
}

func printSuccess(msg string) {
	fmt.Println(bright(msg))
}

// --- Sprite ---

type Sprite struct {
	Name string
	Img  image.Image
	W, H int
	X, Y int // position in the atlas, assigned by pack()
}

// --- JSON output schema ---

type SpriteEntry struct {
	Rect [4]int `json:"rect"` // x, y, width, height in atlas pixels
}

type AtlasFile struct {
	Atlas   string                 `json:"atlas"`
	Size    [2]int                 `json:"size"`
	Sprites map[string]SpriteEntry `json:"sprites"`
}

// --- Main ---

func main() {
	args := os.Args[1:]

	inputDir := ""
	output := "atlas.png"
	padding := 1

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Printf("%s v%s\n", teal("cocoon"), version)
			return
		case "--help", "-h", "help":
			printHelp()
			return
		case "-o", "--output":
			i++
			if i >= len(args) {
				printError("error:", "-o requires a file name")
				os.Exit(1)
			}
			output = args[i]
		case "-p", "--padding":
			i++
			if i >= len(args) {
				printError("error:", "-p requires a pixel count")
				os.Exit(1)
			}
			if _, err := fmt.Sscanf(args[i], "%d", &padding); err != nil || padding < 0 {
				printError("error:", "invalid padding: "+args[i])
				os.Exit(1)
			}
		default:
			if strings.HasPrefix(args[i], "-") {
				printError("error:", "unknown flag: "+args[i])
				os.Exit(1)
			}
			if inputDir != "" {
				printError("error:", "unexpected argument: "+args[i])
				os.Exit(1)
			}
			inputDir = args[i]
		}
	}

	if inputDir == "" {
		printHelp()
		os.Exit(1)
	}

	sprites, err := loadSprites(inputDir)
	if err != nil {
		printError("error:", err)
		os.Exit(1)
	}
	if len(sprites) == 0 {
		printError("error:", "no .png files found in "+inputDir)
		os.Exit(1)
	}

	atlasW, atlasH := pack(sprites, padding)

	if err := checkOverlaps(sprites); err != nil {
		printError("internal error:", err)
		os.Exit(1)
	}

	if err := writeAtlas(sprites, atlasW, atlasH, output); err != nil {
		printError("error:", err)
		os.Exit(1)
	}

	jsonPath := strings.TrimSuffix(output, filepath.Ext(output)) + ".json"
	if err := writeJSON(sprites, atlasW, atlasH, output, jsonPath); err != nil {
		printError("error:", err)
		os.Exit(1)
	}

	occupancy := 0
	for _, s := range sprites {
		occupancy += s.W * s.H
	}
	percent := 100.0 * float64(occupancy) / float64(atlasW*atlasH)

	fmt.Printf("  %s %s\n", teal("atlas"), dim(output))
	fmt.Printf("  %s %s\n", teal("json "), dim(jsonPath))
	printSuccess(fmt.Sprintf("Packed %d sprites into %dx%d (%.0f%% occupancy).",
		len(sprites), atlasW, atlasH, percent))
}

func printHelp() {
	fmt.Printf("%s v%s - a simple sprite atlas packer\n\n", teal("cocoon"), version)
	fmt.Printf("Usage: %s <directory> [flags]\n\n", teal("cocoon"))
	fmt.Printf("Packs all .png files in <directory> into a single atlas image\n")
	fmt.Printf("plus a .json file mapping each sprite name to its pixel rect.\n\n")
	fmt.Printf("Flags:\n")
	fmt.Printf("  %s <file>   Output atlas image (default: atlas.png)\n", teal("-o"))
	fmt.Printf("  %s <px>     Padding between sprites (default: 1)\n", teal("-p"))
	fmt.Printf("  %s      Show this help message\n", teal("--help"))
	fmt.Printf("  %s   Show version\n", teal("--version"))
}

// --- Loading ---

func loadSprites(dir string) ([]*Sprite, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var sprites []*Sprite
	seen := map[string]string{}

	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			continue
		}

		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if prev, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate sprite name %q (%s and %s)", name, prev, e.Name())
		}
		seen[name] = e.Name()

		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		b := img.Bounds()
		sprites = append(sprites, &Sprite{
			Name: name,
			Img:  img,
			W:    b.Dx(),
			H:    b.Dy(),
		})
	}

	return sprites, nil
}

// --- Packing ---

// pack assigns an atlas position to every sprite using shelf packing:
// sprites are sorted tallest-first and laid out left to right in rows
// ("shelves"); when a sprite no longer fits the target width, a new
// shelf starts below the tallest sprite of the previous one. Returns
// the resulting atlas dimensions.
func pack(sprites []*Sprite, padding int) (int, int) {
	sort.Slice(sprites, func(i, j int) bool {
		if sprites[i].H != sprites[j].H {
			return sprites[i].H > sprites[j].H
		}
		return sprites[i].Name < sprites[j].Name
	})

	// Aim for a roughly square atlas: target width is the square root of
	// the total padded area, but never narrower than the widest sprite.
	totalArea := 0
	maxW := 0
	for _, s := range sprites {
		totalArea += (s.W + padding) * (s.H + padding)
		if s.W > maxW {
			maxW = s.W
		}
	}
	targetW := int(math.Ceil(math.Sqrt(float64(totalArea))))
	if targetW < maxW {
		targetW = maxW
	}

	x, y, shelfH := 0, 0, 0
	atlasW := 0
	for _, s := range sprites {
		if x > 0 && x+s.W > targetW {
			x = 0
			y += shelfH + padding
			shelfH = 0
		}
		s.X, s.Y = x, y
		x += s.W + padding
		if s.H > shelfH {
			shelfH = s.H
		}
		if s.X+s.W > atlasW {
			atlasW = s.X + s.W
		}
	}

	return atlasW, y + shelfH
}

// checkOverlaps is a safety net: a correct packer never produces
// overlapping rects, but a bug here would silently corrupt sprites.
func checkOverlaps(sprites []*Sprite) error {
	for i := 0; i < len(sprites); i++ {
		for j := i + 1; j < len(sprites); j++ {
			a, b := sprites[i], sprites[j]
			if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
				return fmt.Errorf("sprites %q and %q overlap", a.Name, b.Name)
			}
		}
	}
	return nil
}

// --- Output ---

func writeAtlas(sprites []*Sprite, w, h int, path string) error {
	atlas := image.NewRGBA(image.Rect(0, 0, w, h))
	for _, s := range sprites {
		dst := image.Rect(s.X, s.Y, s.X+s.W, s.Y+s.H)
		draw.Draw(atlas, dst, s.Img, s.Img.Bounds().Min, draw.Src)
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, atlas)
}

func writeJSON(sprites []*Sprite, w, h int, atlasPath, jsonPath string) error {
	out := AtlasFile{
		Atlas:   filepath.Base(atlasPath),
		Size:    [2]int{w, h},
		Sprites: map[string]SpriteEntry{},
	}
	for _, s := range sprites {
		out.Sprites[s.Name] = SpriteEntry{Rect: [4]int{s.X, s.Y, s.W, s.H}}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonPath, append(data, '\n'), 0o644)
}
