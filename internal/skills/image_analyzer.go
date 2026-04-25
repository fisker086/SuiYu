package skills

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolImageAnalyzer = "builtin_image_analyzer"

var allowedImageOps = map[string]bool{
	"info":       true,
	"exif":       true,
	"dimensions": true,
	"format":     true,
}

func execBuiltinImageAnalyzer(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		op = "info"
	}

	if !allowedImageOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedImageOps)
	}

	filePath := strArg(in, "file_path", "path", "file", "image")
	if filePath == "" {
		return "", fmt.Errorf("missing file_path")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}

	fileSize := stat.Size()
	ext := strings.ToLower(filepath.Ext(filePath))
	formatName := detectImageFormat(ext)

	var b strings.Builder

	switch op {
	case "info":
		cfg, format, err := image.DecodeConfig(file)
		if err != nil {
			b.WriteString(fmt.Sprintf("File: %s\n", filePath))
			b.WriteString(fmt.Sprintf("Size: %s (%d bytes)\n", formatFileSize(fileSize), fileSize))
			b.WriteString(fmt.Sprintf("Format: %s\n", formatName))
			b.WriteString("Note: Could not decode image dimensions\n")
			return b.String(), nil
		}

		aspectRatio := float64(cfg.Width) / float64(cfg.Height)
		b.WriteString(fmt.Sprintf("File: %s\n", filePath))
		b.WriteString(fmt.Sprintf("Size: %s (%d bytes)\n", formatFileSize(fileSize), fileSize))
		b.WriteString(fmt.Sprintf("Format: %s\n", strings.ToUpper(format)))
		b.WriteString(fmt.Sprintf("Dimensions: %dx%d pixels\n", cfg.Width, cfg.Height))
		b.WriteString(fmt.Sprintf("Aspect Ratio: %.2f\n", aspectRatio))
		b.WriteString(fmt.Sprintf("Color Model: %s\n", cfg.ColorModel))

		if aspectRatio > 1.7 && aspectRatio < 1.8 {
			b.WriteString("Resolution: 16:9 (widescreen)\n")
		} else if aspectRatio > 1.3 && aspectRatio < 1.4 {
			b.WriteString("Resolution: 4:3 (standard)\n")
		} else if aspectRatio > 0.5 && aspectRatio < 0.6 {
			b.WriteString("Resolution: 9:16 (portrait/mobile)\n")
		}

	case "dimensions":
		cfg, _, err := image.DecodeConfig(file)
		if err != nil {
			return "", fmt.Errorf("decode image config: %w", err)
		}
		b.WriteString(fmt.Sprintf("Width: %d px\n", cfg.Width))
		b.WriteString(fmt.Sprintf("Height: %d px\n", cfg.Height))
		b.WriteString(fmt.Sprintf("Total Pixels: %d\n", cfg.Width*cfg.Height))
		b.WriteString(fmt.Sprintf("Aspect Ratio: %.2f\n", float64(cfg.Width)/float64(cfg.Height)))

	case "format":
		cfg, format, err := image.DecodeConfig(file)
		if err != nil {
			b.WriteString(fmt.Sprintf("Detected format: %s (by extension)\n", formatName))
			b.WriteString("Note: Could not decode image, may be corrupted or unsupported format\n")
			return b.String(), nil
		}
		b.WriteString(fmt.Sprintf("Format: %s\n", strings.ToUpper(format)))
		b.WriteString(fmt.Sprintf("Color Model: %s\n", cfg.ColorModel))
		b.WriteString(fmt.Sprintf("Bit depth: %s\n", guessBitDepth(fmt.Sprintf("%v", cfg.ColorModel))))

	case "exif":
		b.WriteString(fmt.Sprintf("File: %s\n", filePath))
		b.WriteString(fmt.Sprintf("Size: %s\n", formatFileSize(fileSize)))
		b.WriteString(fmt.Sprintf("Modified: %s\n", stat.ModTime().Format("2006-01-02 15:04:05")))
		b.WriteString("\nNote: EXIF extraction requires external tools (exiftool).\n")
		b.WriteString("This skill provides basic image metadata only.\n")
		b.WriteString("For full EXIF (camera, GPS, timestamp), run: exiftool " + filePath)
	}

	return b.String(), nil
}

func detectImageFormat(ext string) string {
	switch ext {
	case ".jpg", ".jpeg":
		return "JPEG"
	case ".png":
		return "PNG"
	case ".gif":
		return "GIF"
	case ".webp":
		return "WebP"
	case ".bmp":
		return "BMP"
	case ".tiff", ".tif":
		return "TIFF"
	case ".svg":
		return "SVG"
	case ".ico":
		return "ICO"
	default:
		return "Unknown"
	}
}

func formatFileSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	case size < GB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	default:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	}
}

func guessBitDepth(model string) string {
	model = strings.ToLower(model)
	if strings.Contains(model, "rgba") || strings.Contains(model, "nrgba") {
		return "32-bit (RGBA)"
	}
	if strings.Contains(model, "gray") {
		return "8-bit (Grayscale)"
	}
	if strings.Contains(model, "paletted") {
		return "8-bit (Paletted)"
	}
	return "24-bit (RGB)"
}

func NewBuiltinImageAnalyzerTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolImageAnalyzer,
			Desc:  "Analyze image technical details: dimensions, format, file size, aspect ratio. NOT for visual content recognition (use model vision for that).",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: info, exif, dimensions, format", Required: false},
				"file_path": {Type: einoschema.String, Desc: "Path to the image file", Required: true},
			}),
		},
		execBuiltinImageAnalyzer,
	)
}
