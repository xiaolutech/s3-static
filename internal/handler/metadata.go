package handler

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

type fileMetadataResponse struct {
	Path         string    `json:"path"`
	ContentType  string    `json:"contentType,omitempty"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"lastModified"`
	Width        *int      `json:"width"`
	Height       *int      `json:"height"`
}

const (
	mediaWidthHeader  = "x-amz-meta-width"
	mediaHeightHeader = "x-amz-meta-height"
)

func shouldProbeMediaDimensions(path, contentType string) bool {
	mediaType := contentMediaType(contentType)
	if strings.HasPrefix(mediaType, "image/") || strings.HasPrefix(mediaType, "video/") {
		return true
	}

	switch strings.ToLower(fileExtension(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".bmp", ".tif", ".tiff",
		".mp4", ".m4v", ".mov":
		return true
	default:
		return false
	}
}

func extractMediaMetadata(path, contentType string, reader io.Reader) (width int, height int, format string, err error) {
	mediaType := contentMediaType(contentType)
	extension := strings.ToLower(fileExtension(path))

	if mediaType == "image/svg+xml" || extension == ".svg" {
		width, height, err = decodeSVGDimensions(reader)
		return width, height, "svg", err
	}

	if isMP4LikeMedia(mediaType, extension) {
		width, height, err = decodeMP4Dimensions(reader)
		return width, height, "mp4", err
	}

	cfg, format, err := image.DecodeConfig(reader)
	if err != nil {
		return 0, 0, "", err
	}
	return cfg.Width, cfg.Height, format, nil
}

func detectContentTypeFromFormat(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "tiff":
		return "image/tiff"
	case "mp4":
		return "video/mp4"
	default:
		return ""
	}
}

func contentMediaType(contentType string) string {
	mediaType := strings.ToLower(strings.TrimSpace(contentType))
	if idx := strings.Index(mediaType, ";"); idx >= 0 {
		mediaType = mediaType[:idx]
	}
	return strings.TrimSpace(mediaType)
}

func isMP4LikeMedia(mediaType, extension string) bool {
	switch mediaType {
	case "video/mp4", "video/quicktime", "video/x-m4v":
		return true
	}

	switch extension {
	case ".mp4", ".m4v", ".mov":
		return true
	default:
		return false
	}
}

func fileExtension(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx:]
	}
	return ""
}

type svgRoot struct {
	XMLName xml.Name `xml:"svg"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	ViewBox string   `xml:"viewBox,attr"`
}

func decodeSVGDimensions(reader io.Reader) (int, int, error) {
	decoder := xml.NewDecoder(io.LimitReader(reader, 32*1024))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return 0, 0, fmt.Errorf("svg root not found")
			}
			return 0, 0, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "svg" {
			continue
		}

		var root svgRoot
		if err := decoder.DecodeElement(&root, &start); err != nil {
			return 0, 0, err
		}

		if width, ok := parseSVGLength(root.Width); ok {
			if height, ok := parseSVGLength(root.Height); ok {
				return width, height, nil
			}
		}

		if width, height, ok := parseSVGViewBox(root.ViewBox); ok {
			return width, height, nil
		}

		return 0, 0, fmt.Errorf("svg dimensions unavailable")
	}
}

func parseSVGLength(value string) (int, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "px")
	if value == "" {
		return 0, false
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return int(f + 0.5), true
}

func parseSVGViewBox(value string) (int, int, bool) {
	fields := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	if len(fields) != 4 {
		return 0, 0, false
	}

	width, err := strconv.ParseFloat(fields[2], 64)
	if err != nil || width <= 0 {
		return 0, 0, false
	}

	height, err := strconv.ParseFloat(fields[3], 64)
	if err != nil || height <= 0 {
		return 0, 0, false
	}

	return int(width + 0.5), int(height + 0.5), true
}

func decodeMP4Dimensions(reader io.Reader) (int, int, error) {
	data, err := io.ReadAll(io.LimitReader(reader, 4*1024*1024))
	if err != nil {
		return 0, 0, err
	}

	width, height, ok := findMP4TrackDimensions(data, 0, len(data))
	if !ok {
		return 0, 0, fmt.Errorf("mp4 dimensions unavailable")
	}
	return width, height, nil
}

func findMP4TrackDimensions(data []byte, start, end int) (int, int, bool) {
	for offset := start; offset+8 <= end; {
		size, boxType, headerSize, ok := readMP4BoxHeader(data, offset, end)
		if !ok {
			break
		}

		boxEnd := offset + size
		if isMP4ContainerBox(boxType) {
			childStart := offset + headerSize
			if boxType == "meta" {
				childStart += 4
			}
			if childStart < boxEnd {
				if width, height, ok := findMP4TrackDimensions(data, childStart, boxEnd); ok {
					return width, height, true
				}
			}
		}

		if boxType == "tkhd" {
			if width, height, ok := parseTKHDBox(data[offset+headerSize : boxEnd]); ok {
				return width, height, true
			}
		}

		offset = boxEnd
	}

	return 0, 0, false
}

func readMP4BoxHeader(data []byte, offset, end int) (size int, boxType string, headerSize int, ok bool) {
	if offset+8 > end {
		return 0, "", 0, false
	}

	rawSize := binary.BigEndian.Uint32(data[offset : offset+4])
	boxType = string(data[offset+4 : offset+8])
	headerSize = 8

	switch rawSize {
	case 0:
		size = end - offset
	case 1:
		if offset+16 > end {
			return 0, "", 0, false
		}
		largeSize := binary.BigEndian.Uint64(data[offset+8 : offset+16])
		if largeSize > uint64(math.MaxInt) {
			return 0, "", 0, false
		}
		size = int(largeSize)
		headerSize = 16
	default:
		size = int(rawSize)
	}

	if size < headerSize || offset+size > end {
		return 0, "", 0, false
	}

	return size, boxType, headerSize, true
}

func isMP4ContainerBox(boxType string) bool {
	switch boxType {
	case "moov", "trak", "mdia", "minf", "stbl", "edts", "udta", "meta":
		return true
	default:
		return false
	}
}

func parseTKHDBox(payload []byte) (int, int, bool) {
	if len(payload) < 8 {
		return 0, 0, false
	}

	version := payload[0]
	var widthOffset int
	switch version {
	case 0:
		widthOffset = 76
	case 1:
		widthOffset = 88
	default:
		return 0, 0, false
	}

	if len(payload) < widthOffset+8 {
		return 0, 0, false
	}

	width := int(binary.BigEndian.Uint32(payload[widthOffset:widthOffset+4]) >> 16)
	height := int(binary.BigEndian.Uint32(payload[widthOffset+4:widthOffset+8]) >> 16)
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}
