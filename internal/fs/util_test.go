package fs

import (
	"testing"

	"github.com/tableauio/tableau/format"
)

func TestCopyFile(t *testing.T) {
	type args struct {
		src string
		dst string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				src: "./testdata/empty.txt",
				dst: "./testdata/_empty.txt",
			},
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				src: "./testdata/not-exist-empty.txt",
				dst: "./testdata/_empty.txt",
			},
			wantErr: true,
		},
		{
			name: "test3",
			args: args{
				src: "./testdata/empty.txt",
				dst: "./testdata/empty.txt",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CopyFile(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("CopyFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_copyFileContents(t *testing.T) {
	type args struct {
		src string
		dst string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				src: "./testdata/test.txt",
				dst: "./testdata/_test1.txt",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := copyFileContents(tt.args.src, tt.args.dst); (err != nil) != tt.wantErr {
				t.Errorf("copyFileContents() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExists(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				path: "./testdata/test.txt",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "test2",
			args: args{
				path: "./testdata/test2.txt",
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Exists(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Exists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Exists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasSubdirPrefix(t *testing.T) {
	type args struct {
		path    string
		subdirs []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{
				path: "./testdata/test.txt",
				subdirs: []string{
					"./testdata/",
				},
			},
			want: true,
		},
		{
			name: "test2",
			args: args{
				path: "./testdata/test.txt",
				subdirs: []string{
					"testdata/",
				},
			},
			want: true,
		},
		{
			name: "test3",
			args: args{
				path: "./testdata/test.txt",
				subdirs: []string{
					"testdataXX/",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSubdirPrefix(tt.args.path, tt.args.subdirs); got != tt.want {
				t.Errorf("HasSubdirPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRewriteSubdir(t *testing.T) {
	type args struct {
		path       string
		subdirRewrites map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{
				path: "./testdata/test.txt",
				subdirRewrites: map[string]string{
					"testdata/": "testdataXX/",
				},
			},
			want: "testdataXX/test.txt",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RewriteSubdir(tt.args.path, tt.args.subdirRewrites); got != tt.want {
				t.Errorf("RewriteSubdir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRangeFilesByFormat(t *testing.T) {
	type args struct {
		dir      string
		fmt      format.Format
		callback func(bookPath string) error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test excel",
			args: args{
				dir: "./testdata/",
				fmt: format.Excel,
				callback: func(bookPath string) error {
					return nil
				},
			},
			wantErr: false,
		},
		{
			name: "test csv",
			args: args{
				dir: "./testdata/",
				fmt: format.CSV,
				callback: func(bookPath string) error {
					return nil
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := RangeFilesByFormat(tt.args.dir, tt.args.fmt, tt.args.callback); (err != nil) != tt.wantErr {
				t.Errorf("RangeFilesByFormat() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
