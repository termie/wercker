// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package core

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/wercker/wercker/util"
)

// NewFileStore creates a new FileStore
func NewFileStore(options *PipelineOptions, storepath string) *FileStore {

	logger := util.RootLogger().WithField("Logger", "FileStore")

	return &FileStore{
		storepath: storepath,
		logger:    logger,
		options:   options,
	}
}

// FileStore stores files on the local file system
type FileStore struct {
	storepath string
	logger    *util.LogEntry
	options   *PipelineOptions
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *FileStore) StoreFromFile(args *StoreFromFileArgs) error {

	s.logger.WithFields(util.LogFields{
		"Bucket": s.options.S3Bucket,
		"Path":   args.Path,
		"S3Key":  args.Key,
	}).Info("Uploading file to local-file-store")

	var reader io.ReadCloser

	reader, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open input file")
		return err
	}

	// Ensure output directory exists for this file
	outputFile := fmt.Sprintf("%s/%s/%s", s.storepath, s.options.S3Bucket, args.Key)
	i := strings.LastIndex(outputFile, "/")
	if i == -1 {
		panic(fmt.Sprintf("invalid file descriptor: %s", outputFile))
	}
	outputDirt := outputFile[0:i]
	err = os.MkdirAll(outputDirt, 0700)
	if err != nil {
		return errors.Wrap(err, "unable to create local destination directory")
	}
	writer, err := os.Create(outputFile)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create destination file %s", outputFile))
	}
	_, err = io.Copy(writer, reader)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to store to file %s", outputFile))
	}
	writer.Close()

	s.logger.Debugf("Completed store operation %s", outputFile)

	return nil
}
