package gencontent

import (
	"testing"

	"github.com/hamed/aistudio-api/internal/chatgptweb"
)

func TestValidateGeneratedImage(t *testing.T) {
	tests := []struct {
		name    string
		image   chatgptweb.Image
		want    string
		wantErr bool
	}{
		{
			name:  "valid PNG",
			image: chatgptweb.Image{MIMEType: "image/png", Data: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="},
			want:  "image/png",
		},
		{name: "invalid base64", image: chatgptweb.Image{MIMEType: "image/png", Data: "%%%"}, wantErr: true},
		{name: "not an image", image: chatgptweb.Image{MIMEType: "image/png", Data: "dGV4dA=="}, wantErr: true},
		{name: "MIME mismatch", image: chatgptweb.Image{MIMEType: "image/jpeg", Data: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateGeneratedImage(test.image)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("got=%q err=%v want=%q wantErr=%t", got, err, test.want, test.wantErr)
			}
		})
	}
}
