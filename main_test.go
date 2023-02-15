package main

import (
	"reflect"
	"testing"
)

func Test_moveImageReferences(t *testing.T) {
	type args struct {
		content  []byte
		filename string
		verbose  bool
		from     string
		to       string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		want1   bool
		wantErr bool
	}{
		{
			"simple",
			args{[]byte(`FROM python:3.7`),
				"Dockerfile",
				true,
				"index.docker.io/library",
				"public.ecr.aws/docker/library",
			},
			[]byte(`FROM public.ecr.aws/docker/library/python:3.7`),
			true,
			false,
		},
		{
			"and back again",
			args{[]byte(`FROM public.ecr.aws/docker/library/python:3.7`),
				"Dockerfile",
				true,
				"public.ecr.aws/docker/library",
				"index.docker.io/library",
			},
			[]byte(`FROM python:3.7`),
			true,
			false,
		},

		{
			"error",
			args{[]byte(`FROM python:3.7`),
				"Dockerfile",
				true,
				"index.docker.io/library",
				"public.ecr.aws/dorker/library",
			},
			nil,
			false,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := moveImageReferences(tt.args.content, tt.args.filename, tt.args.verbose, tt.args.from, tt.args.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("moveImageReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("moveImageReferences() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("moveImageReferences() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
