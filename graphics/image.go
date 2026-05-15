package graphics

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"strings"
)

// LoadImage decodes PNG, JPEG, or GIF bytes into an image.Image.
func LoadImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// LoadImageURI loads an image from a filesystem path, file URI, or data URI.
func LoadImageURI(uri string) (image.Image, error) {
	data, err := imageBytesFromURI(uri)
	if err != nil {
		return nil, err
	}
	return LoadImage(data)
}

// NewTextureFromFile loads an image file and uploads it as a texture.
func NewTextureFromFile(win Window, path string) (Texture, error) {
	if win == nil {
		return nil, fmt.Errorf("nil window")
	}
	img, err := LoadImageURI(path)
	if err != nil {
		return nil, err
	}
	return win.NewTexture(img)
}

// NewTextureFromURI loads a filesystem path, file URI, or data URI and uploads it.
// Callers that already hold a Texture can pass it directly to Frame.RenderQuad,
// Frame.RenderMaskedQuad, Window.NewMesh, or DrawOptions.Mask.
func NewTextureFromURI(win Window, uri string) (Texture, error) {
	return NewTextureFromFile(win, uri)
}

func imageBytesFromURI(uri string) ([]byte, error) {
	if strings.HasPrefix(uri, "data:") {
		comma := strings.IndexByte(uri, ',')
		if comma < 0 {
			return nil, fmt.Errorf("invalid data URI")
		}
		meta := uri[:comma]
		payload := uri[comma+1:]
		if strings.HasSuffix(meta, ";base64") {
			return base64.StdEncoding.DecodeString(payload)
		}
		decoded, err := url.QueryUnescape(payload)
		if err != nil {
			return nil, err
		}
		return []byte(decoded), nil
	}

	path := uri
	if strings.HasPrefix(uri, "file://") {
		u, err := url.Parse(uri)
		if err != nil {
			return nil, err
		}
		path = u.Path
	}
	return os.ReadFile(path)
}
