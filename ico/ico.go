package ico

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
)

type IconDir struct {
	Reserved uint16 // must be 0
	Type     uint16 // 1 for ICO, 2 for CUR
	Count    uint16 // number of images
}

type IconDirEntry struct {
	Width        uint8 // 0 means 256
	Height       uint8 // 0 means 256
	ColorCount   uint8
	Reserved     uint8  // must be 0
	ColorPlanes  uint16 // 0 or 1
	BitsPerPixel uint16
	SizeInBytes  uint32
	Offset       uint32
}

func ImageToIco(img image.Image) ([]byte, error) {
	// encode image as png
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	pngData := pngBuf.Bytes()

	// get image dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// prepare output buffer
	var icoBuf bytes.Buffer

	iconDir := IconDir{
		Reserved: 0,
		Type:     1, // ICO format
		Count:    1, // One image
	}

	w := uint8(width)
	h := uint8(height)

	// width/height of 256 should be stored as 0
	if width == 256 {
		w = 0
	}
	if height == 256 {
		h = 0
	}

	iconDirEntry := IconDirEntry{
		Width:        w,
		Height:       h,
		ColorCount:   0, // 0 for PNG (32-bit)
		Reserved:     0,
		ColorPlanes:  1,
		BitsPerPixel: 32, // PNG with alpha
		SizeInBytes:  uint32(len(pngData)),
		Offset:       6 + 16, // Size of ICONDIR + ICONDIRENTRY
	}

	// write IconDir
	if err := binary.Write(&icoBuf, binary.LittleEndian, iconDir); err != nil {
		return nil, fmt.Errorf("failed to write ICONDIR: %w", err)
	}

	// write IconDirEntry
	if err := binary.Write(&icoBuf, binary.LittleEndian, iconDirEntry); err != nil {
		return nil, fmt.Errorf("failed to write ICONDIRENTRY: %w", err)
	}

	// write PNG data directly
	if _, err := icoBuf.Write(pngData); err != nil {
		return nil, fmt.Errorf("failed to write PNG data: %w", err)
	}

	return icoBuf.Bytes(), nil
}
