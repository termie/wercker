// Copyright (c) 2018, Oracle and/or its affiliates. All rights reserved.

package core

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	ocisdkcomm "github.com/oracle/oci-go-sdk/common"
	ocisdkstorage "github.com/oracle/oci-go-sdk/objectstorage"
	"github.com/wercker/pkg/log"
	"github.com/wercker/wercker/util"
)

// OciEnvVarPrefix is the prefix to use for all environment variables needed by the OCI API
const OciEnvVarPrefix = "WERCKER_OCI_"

const TenancyVarName = "TENANCY_OCID"
const UserVarName = "USER_OCID"
const RegionVarName = "REGION"
const PrivateKeyVarName = "PRIVATE_KEY_PATH"
const FingerPrintVarName = "FINGERPRINT"
const PrivateKeyPassphraseVarName = "PRIVATE_KEY_PASSPHRASE"

// NewObjectStore creates a new OCI Object Store
func NewObjectStore(options *OCIOptions) *ObjectStore {

	logger := util.RootLogger().WithField("Logger", "ObjectStore")
	if options == nil {
		logger.Panic("options cannot be nil")
	}

	return &ObjectStore{
		logger:  logger,
		options: options,
	}
}

// ObjectStore stores files in OCI Object Store
type ObjectStore struct {
	session *session.Session
	logger  *util.LogEntry
	options *OCIOptions
}

// StoreFromFile copies the file from args.Path to options.Bucket + args.Key.
func (s *ObjectStore) StoreFromFile(args *StoreFromFileArgs) error {
	if args.MaxTries == 0 {
		args.MaxTries = 1
	}

	s.logger.WithFields(util.LogFields{
		"Bucket":   s.options.Bucket,
		"Path":     args.Path,
		"Region":   s.options.Region,
		"Key":      args.Key,
		"MaxTries": args.MaxTries,
	}).Info("Uploading file to OCI Object Store")

	var content io.ReadCloser

	content, err := os.Open(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to open input file")
		return err
	}

	fileStat, err := os.Stat(args.Path)
	if err != nil {
		s.logger.WithField("Error", err).Error("Unable to stat input file")
		return err
	}

	//OCI SDK requires int content length
	contentLength := fileStat.Size()
	var outerErr error

	for try := 1; try <= args.MaxTries; try++ {

		err = putObject(s.options, args.Key, content, contentLength)
		if err != nil {
			s.logger.WithFields(util.LogFields{
				"Bucket":   s.options.Bucket,
				"Path":     args.Path,
				"Region":   s.options.Region,
				"Key":      args.Key,
				"Try":      try,
				"MaxTries": args.MaxTries,
			}).Error("Unable to upload file to OCI object store")
			outerErr = err
			continue
		}

		s.logger.WithFields(util.LogFields{
			"Bucket":   s.options.Bucket,
			"Path":     args.Path,
			"Region":   s.options.Region,
			"Key":      args.Key,
			"Try":      try,
			"MaxTries": args.MaxTries,
		}).Info("Uploading file to OCI object store complete")

		return nil
	}

	return outerErr
}

func getOciEnvVar(varName string) (string, error) {
	envVarName := OciEnvVarPrefix + varName
	val := os.Getenv(envVarName)
	if val == "" {
		return "", fmt.Errorf("environment variable %s must be set", envVarName)
	} else {
		return val, nil
	}
}

func getOciPrivateKeyPassphrase() string {
	if value, ok := os.LookupEnv(OciEnvVarPrefix + PrivateKeyPassphraseVarName); !ok {
		return ""
	} else {
		return value
	}
}

func createOciObjectStoreClient() (client ocisdkstorage.ObjectStorageClient, err error) {
	privateKeyPassphrase := getOciPrivateKeyPassphrase()
	ociTenancy, err := getOciEnvVar(TenancyVarName)
	if err != nil {
		return
	}

	ociUser, err := getOciEnvVar(UserVarName)
	if err != nil {
		return
	}

	ociRegion, err := getOciEnvVar(RegionVarName)
	if err != nil {
		return
	}

	ociFingerprint, err := getOciEnvVar(FingerPrintVarName)
	if err != nil {
		return
	}

	ociPrivateKeyFile, err := getOciEnvVar(PrivateKeyVarName)
	if err != nil {
		return
	}

	ociPrivateKeyBytes, err := ioutil.ReadFile(ociPrivateKeyFile)
	if err != nil {
		return
	}

	configProv := ocisdkcomm.NewRawConfigurationProvider(
		ociTenancy,
		ociUser,
		ociRegion,
		ociFingerprint,
		string(ociPrivateKeyBytes[:]),
		&privateKeyPassphrase,
	)
	client, err = ocisdkstorage.NewObjectStorageClientWithConfigurationProvider(configProv)
	return
}

func putObject(options *OCIOptions, objectName string, reader io.ReadCloser, contentLength int64) error {
	objStoreClient, err := createOciObjectStoreClient()
	if err != nil {
		return err
	}

	putRequest := ocisdkstorage.PutObjectRequest{
		NamespaceName: &options.Namespace,
		BucketName:    &options.Bucket,
		ObjectName:    &objectName,
		PutObjectBody: reader,
		ContentLength: &contentLength,
	}

	if err != nil {
		return err
	}

	resp, err := objStoreClient.PutObject(context.Background(), putRequest)
	if err != nil {
		return err
	}
	log.Debugf("Completed put object %s in bucket %s. Response from server is: %s",
		objectName, options.Bucket, resp)
	return nil
}
