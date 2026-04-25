package storage

import (
	"reflect"
	"testing"
)

func TestChatHistoryAttachmentsFromExtraJSONB(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		raw       string
		wantImg   []string
		wantFiles []string
	}{
		{
			name:      "empty",
			raw:       `{}`,
			wantImg:   nil,
			wantFiles: nil,
		},
		{
			name:      "arrays",
			raw:       `{"image_urls":["https://a/x.png"],"file_urls":["https://a/f.pdf"]}`,
			wantImg:   []string{"https://a/x.png"},
			wantFiles: []string{"https://a/f.pdf"},
		},
		{
			name:      "single_string_image_urls",
			raw:       `{"image_urls":"https://a/one.jpg"}`,
			wantImg:   []string{"https://a/one.jpg"},
			wantFiles: nil,
		},
		{
			name:      "legacy_image_url",
			raw:       `{"image_url":"https://a/legacy.png"}`,
			wantImg:   []string{"https://a/legacy.png"},
			wantFiles: nil,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotImg, gotFiles := ChatHistoryAttachmentsFromExtraJSONB([]byte(tc.raw))
			if !reflect.DeepEqual(gotImg, tc.wantImg) {
				t.Fatalf("image: got %#v want %#v", gotImg, tc.wantImg)
			}
			if !reflect.DeepEqual(gotFiles, tc.wantFiles) {
				t.Fatalf("files: got %#v want %#v", gotFiles, tc.wantFiles)
			}
		})
	}
}
