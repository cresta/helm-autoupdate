package helm

import (
	"bytes"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"helm.sh/helm/v3/pkg/getter"
)

func S3Provider() getter.Provider {
	return getter.Provider{
		Schemes: []string{"s3"},
		New: func(opts ...getter.Option) (getter.Getter, error) {
			return NewS3Getter(opts)
		},
	}
}

type s3Getter struct {
	s3Client *s3.S3
}

func (s *s3Getter) Get(s3Url string, _ ...getter.Option) (*bytes.Buffer, error) {
	u, err := url.Parse(s3Url)
	if err != nil {
		return nil, fmt.Errorf("invalid s3 s3Url format: %s", s3Url)
	}
	bucket := u.Host
	key := u.Path
	if key == "" {
		key = "/"
	}
	if key[0] == '/' {
		key = key[1:]
	}
	params := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	resp, err := s.s3Client.GetObject(params)
	if err != nil {
		return nil, fmt.Errorf("unable to get s3 object: %w", err)
	}
	var ret bytes.Buffer
	_, err = ret.ReadFrom(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read s3 object: %w", err)
	}
	return &ret, nil
}

func NewS3Getter(_ []getter.Option) (getter.Getter, error) {
	ses, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to get s3 session: %w", err)
	}
	s3Client := s3.New(ses)
	return &s3Getter{s3Client: s3Client}, nil
}
