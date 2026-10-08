package studio

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jvstin47/primitive_svg_gen/primitive"
	"github.com/nfnt/resize"
	_ "golang.org/x/image/webp"
)

//go:embed index.html
var indexHTML []byte

type GenerateRequest struct {
	ImageBase64   string  `json:"image"`          // data:image/...;base64,... or raw base64
	ExampleName   string  `json:"example"`        // owl, monalisa, lenna, pyramids
	Mode          int     `json:"mode"`           // 0=combo, 1=triangle, 2=rect, 3=ellipse, 4=circle, 5=rotatedrect, 6=beziers, 7=rotatedellipse, 8=polygon, 9=line
	Count         int     `json:"count"`          // number of shapes
	Alpha         int     `json:"alpha"`          // 0 for auto, 1-255
	Repeat        int     `json:"repeat"`         // extra shapes per iteration
	InputSize     int     `json:"inputSize"`      // resize input to this bounding box
	OutputSize    int     `json:"outputSize"`     // output svg dimensions
	Background    string  `json:"background"`     // "avg", "transparent", or hex color
	Palette       string  `json:"palette"`        // "original", "grayscale", "duotone", "cyberpunk", "sunset", "sepia", "matrix", "monochrome", "invert"
	Duotone1      string  `json:"duotone1"`       // hex
	Duotone2      string  `json:"duotone2"`       // hex
	StrokeOnly    bool    `json:"strokeOnly"`     // wireframe / outline mode
	StrokeWidth   float64 `json:"strokeWidth"`    // stroke width for outlines
	GroupLayers   bool    `json:"groupLayers"`    // wrap in <g id="primitive-N">
}

type GenerateResponse struct {
	SVG      string  `json:"svg"`
	Score    float64 `json:"score"`
	Elapsed  float64 `json:"elapsed"`
	Shapes   int     `json:"shapes"`
	ModeName string  `json:"modeName"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	SVGSize  int     `json:"svgSize"`
	Error    string  `json:"error,omitempty"`
}

type VariationItem struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Response    GenerateResponse `json:"result"`
}

type VariationsResponse struct {
	Variations []VariationItem `json:"variations"`
	Error      string          `json:"error,omitempty"`
}

func shapeName(mode int) string {
	names := map[int]string{
		0: "Combo",
		1: "Triangles",
		2: "Rectangles",
		3: "Ellipses",
		4: "Circles",
		5: "Rotated Rectangles",
		6: "Bézier Curves",
		7: "Rotated Ellipses",
		8: "Polygons",
		9: "Lines / Strokes",
	}
	if name, ok := names[mode]; ok {
		return name
	}
	return "Custom"
}

func decodeImage(req GenerateRequest) (image.Image, error) {
	if req.ExampleName != "" {
		examplePath := filepath.Join("examples", req.ExampleName+".png")
		data, err := os.ReadFile(examplePath)
		if err != nil {
			return nil, fmt.Errorf("example not found: %s", req.ExampleName)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		return img, err
	}

	dataStr := req.ImageBase64
	if idx := strings.Index(dataStr, ","); idx != -1 {
		dataStr = dataStr[idx+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base64 image data: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}
	return img, nil
}

func runPrimitiveModel(img image.Image, req GenerateRequest) (*GenerateResponse, error) {
	if req.Count <= 0 {
		req.Count = 50
	}
	if req.Count > 1000 {
		req.Count = 1000
	}
	if req.InputSize <= 0 {
		req.InputSize = 256
	}
	if req.OutputSize <= 0 {
		req.OutputSize = 1024
	}

	// Downsample input for fast computation
	input := resize.Thumbnail(uint(req.InputSize), uint(req.InputSize), img, resize.Bilinear)

	// Determine background color
	var bg primitive.Color
	bgChoice := strings.ToLower(strings.TrimSpace(req.Background))
	if bgChoice == "" || bgChoice == "avg" || bgChoice == "average" {
		bg = primitive.MakeColor(primitive.AverageImageColor(input))
	} else if bgChoice == "transparent" {
		bg = primitive.Color{0, 0, 0, 0}
	} else {
		bg = primitive.MakeHexColor(bgChoice)
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	model := primitive.NewModel(input, bg, req.OutputSize, workers)
	startTime := time.Now()

	for i := 0; i < req.Count; i++ {
		model.Step(primitive.ShapeType(req.Mode), req.Alpha, req.Repeat)
	}

	elapsed := time.Since(startTime).Seconds()

	var d1, d2 primitive.Color
	if req.Duotone1 != "" {
		d1 = primitive.MakeHexColor(req.Duotone1)
	} else {
		d1 = primitive.MakeHexColor("#000000")
	}
	if req.Duotone2 != "" {
		d2 = primitive.MakeHexColor(req.Duotone2)
	} else {
		d2 = primitive.MakeHexColor("#ffffff")
	}

	svgOpts := primitive.SVGOptions{
		ViewBox:       true,
		TransparentBG: bgChoice == "transparent",
		StrokeOnly:    req.StrokeOnly,
		StrokeWidth:   req.StrokeWidth,
		Palette:       req.Palette,
		Duotone1:      d1,
		Duotone2:      d2,
		GroupLayers:   req.GroupLayers,
	}

	svgContent := model.SVGWithOptions(svgOpts)

	return &GenerateResponse{
		SVG:      svgContent,
		Score:    model.Score,
		Elapsed:  elapsed,
		Shapes:   req.Count,
		ModeName: shapeName(req.Mode),
		Width:    model.Sw,
		Height:   model.Sh,
		SVGSize:  len(svgContent),
	}, nil
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	img, err := decodeImage(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GenerateResponse{Error: err.Error()})
		return
	}

	resp, err := runPrimitiveModel(img, req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GenerateResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleVariations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var baseReq GenerateRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &baseReq); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	img, err := decodeImage(baseReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(VariationsResponse{Error: err.Error()})
		return
	}

	// 4 distinct artistic interpretations
	presets := []struct {
		title string
		desc  string
		req   GenerateRequest
	}{
		{
			title: "Modernist Triangles",
			desc:  "Geometric polygons forming sharp crystalline structures",
			req: GenerateRequest{
				Mode:       1, // Triangle
				Count:      60,
				Alpha:      128,
				InputSize:  256,
				OutputSize: 1024,
				Palette:    "original",
			},
		},
		{
			title: "Pointillist Circles",
			desc:  "Soft overlapping circles mimicking impressionist pointillism",
			req: GenerateRequest{
				Mode:       4, // Circle
				Count:      80,
				Alpha:      128,
				InputSize:  256,
				OutputSize: 1024,
				Palette:    "original",
			},
		},
		{
			title: "Cyberpunk Curves",
			desc:  "Fluid Bézier curves in high-contrast neon magenta and cyan",
			req: GenerateRequest{
				Mode:        6, // Béziers
				Count:       70,
				Alpha:       160,
				InputSize:   256,
				OutputSize:  1024,
				Palette:     "cyberpunk",
				Background:  "#0b001a",
				StrokeWidth: 2.0,
			},
		},
		{
			title: "Architectural Wireframes",
			desc:  "Minimalist stroked outlines revealing structural forms",
			req: GenerateRequest{
				Mode:        5, // Rotated Rectangles
				Count:       65,
				Alpha:       200,
				InputSize:   256,
				OutputSize:  1024,
				StrokeOnly:  true,
				StrokeWidth: 1.5,
				Palette:     "original",
			},
		},
	}

	results := make([]VariationItem, len(presets))
	var wg sync.WaitGroup

	for i, p := range presets {
		wg.Add(1)
		go func(idx int, pr GenerateRequest, title, desc string) {
			defer wg.Done()
			res, err := runPrimitiveModel(img, pr)
			if err != nil {
				results[idx] = VariationItem{
					Title:       title,
					Description: desc,
					Response:    GenerateResponse{Error: err.Error()},
				}
				return
			}
			results[idx] = VariationItem{
				Title:       title,
				Description: desc,
				Response:    *res,
			}
		}(i, p.req, p.title, p.desc)
	}

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(VariationsResponse{Variations: results})
}

func handleExample(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/examples/")
	name = strings.TrimSuffix(name, ".png")
	examplePath := filepath.Join("examples", name+".png")
	data, err := os.ReadFile(examplePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
}

// StartServer starts the HTTP server on the given address
func StartServer(addr string) error {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexHTML)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/api/generate", handleGenerate)
	mux.HandleFunc("/api/variations", handleVariations)
	mux.HandleFunc("/api/examples/", handleExample)

	log.Printf("✦ Primitive SVG Studio listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
