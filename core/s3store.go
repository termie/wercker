//   Copyright 2016 Wercker Holding BV
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package core

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/wercker/wercker/util"
)

// NewS3Store creates a new S3Store
func NewS3Store(options *AWSOptions) *S3Store {

	logger := util.RootLogger().WithField("Logger", "S3Store")
	if options == nil {
		logger.Panic("options cannot be nil")
	}
	conf := aws.NewConfig()
	if options.AWSAccessKeyID != "" && options.AWSSecretAccessKey != "" {
		creds := credentials.NewStaticCredentials(options.AWSAccessKeyID, options.AWSSecretAccessKey, "")
		conf = conf.WithCredentials(creds)
	}
	conf = conf.WithRegion(options.AWSRegion)
	sess := session.New(conf)

	return &S3Store{
		session: sess,
		logger:  logger,
		options: options,
	}
}

// S3Store stores files in S3
type S3Store struct {
	session *session.Session
	logger  *util.LogEntry
	options *AWSOptions
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *S3Store) StoreFromFile(args *StoreFromFileArgs) error {
	if args.MaxTries == 0 {
		args.MaxTries = 1
	}

	s.logger.WithFields(util.LogFields{
		"Bucket":   s.options.S3Bucket,
		"Path":     args.Path,
		"Region":   s.options.AWSRegion,
		"S3Key":    args.Key,
		"MaxTries": args.MaxTries,
	}).Info("Uploading file to S3")

	file, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open input file")
		return err
	}
	defer file.Close()

	var outerErr error
	uploadManager := s3manager.NewUploader(s.session, func(u *s3manager.Uploader) {
		u.PartSize = s.options.S3PartSize
	})
	for try := 1; try <= args.MaxTries; try++ {

		_, err = uploadManager.Upload(&s3manager.UploadInput{
			ACL:                  aws.String("private"),
			Body:                 file,
			Bucket:               aws.String(s.options.S3Bucket),
			Key:                  aws.String(args.Key),
			Metadata:             args.Meta,
			ServerSideEncryption: aws.String("AES256"),
		})

		if err != nil {
			s.logger.WithFields(util.LogFields{
				"Bucket":   s.options.S3Bucket,
				"Path":     args.Path,
				"Region":   s.options.AWSRegion,
				"S3Key":    args.Key,
				"Try":      try,
				"MaxTries": args.MaxTries,
			}).Error("Unable to upload file to S3")
			outerErr = err
			continue
		}

		s.logger.WithFields(util.LogFields{
			"Bucket":   s.options.S3Bucket,
			"Path":     args.Path,
			"Region":   s.options.AWSRegion,
			"S3Key":    args.Key,
			"Try":      try,
			"MaxTries": args.MaxTries,
		}).Info("Uploading file to S3 complete")

		return nil
	}

	return outerErr
}
