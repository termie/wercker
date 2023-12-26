package dockerauth

import (
	"errors"
	"net/url"
	"strings"

	"github.com/wercker/docker-check-access"
	"github.com/wercker/wercker/util"
)

type CheckAccessOptions struct {
	Username               string `yaml:"username"`
	Password               string `yaml:"password"`
	Registry               string `yaml:"registry"`
	AwsRegistryID          string `yaml:"aws-registry-id"`
	AwsRegion              string `yaml:"aws-region"`
	AwsAccessKey           string `yaml:"aws-access-key"`
	AwsSecretKey           string `yaml:"aws-secret-key"`
	AwsStrictAuth          bool   `yaml:"aws-strict-auth"`
	AzureLoginServer       string `yaml:"azure-login-server"`
	AzureRegistryName      string `yaml:"azure-registry-name"`
	AzureClientID          string `yaml:"azure-client-id"`
	AzureClientSecret      string `yaml:"azure-client-secret"`
	AzureSubscriptionID    string `yaml:"azure-subscription-id"`
	AzureTenantID          string `yaml:"azure-tenant-id"`
	AzureResourceGroupName string `yaml:"azure-resource-group"`
}

func (a *CheckAccessOptions) Interpolate(env *util.Environment) {
	a.Username = env.Interpolate(a.Username)
	a.Password = env.Interpolate(a.Password)
	a.Registry = env.Interpolate(a.Registry)
	a.AwsRegistryID = env.Interpolate(a.AwsRegistryID)
	a.AwsRegion = env.Interpolate(a.AwsRegion)
	a.AwsAccessKey = env.Interpolate(a.AwsAccessKey)
	a.AwsSecretKey = env.Interpolate(a.AwsSecretKey)
	a.AzureLoginServer = env.Interpolate(a.AzureLoginServer)
	a.AzureRegistryName = env.Interpolate(a.AzureRegistryName)
	a.AzureClientID = env.Interpolate(a.AzureClientID)
	a.AzureClientSecret = env.Interpolate(a.AzureClientSecret)
	a.AzureSubscriptionID = env.Interpolate(a.AzureSubscriptionID)
	a.AzureTenantID = env.Interpolate(a.AzureTenantID)
	a.AzureResourceGroupName = env.Interpolate(a.AzureResourceGroupName)
}

const (
	DockerRegistryV2 = "https://index.docker.io/v2/"
)

var ErrNoAuthenticator = errors.New("Unable to make authenticator for this registry")

func NormalizeRegistry(address string) string {
	logger := util.RootLogger().WithField("Logger", "Docker")
	if address == "" {
		logger.Debugln("No registry address provided, using https://registry.hub.docker.com")
		return DockerRegistryV2
	}

	parsed, err := url.Parse(address)
	if err != nil {
		logger.Errorln("Registry address is invalid, this will probably fail:", address)
		return address
	}
	if parsed.Scheme != "https" {
		logger.Warnln("Registry address is expected to begin with 'https://', forcing it to use https")
		parsed.Scheme = "https"
		address = parsed.String()
	}
	if strings.HasSuffix(address, "/") {
		address = address[:len(address)-1]
	}

	parts := strings.Split(address, "/")
	possiblyAPIVersionStr := parts[len(parts)-1]

	// send them a v1 registry if they don't specify
	if possiblyAPIVersionStr != "v1" && possiblyAPIVersionStr != "v2" {
		newParts := append(parts, "v1")
		address = strings.Join(newParts, "/")
	}
	return address + "/"
}

func GetRegistryAuthenticator(opts CheckAccessOptions) (auth.Authenticator, error) {
	//calls to this function probably already have normalized registries, but call it again jic
	reg := NormalizeRegistry(opts.Registry)

	//try to get domain and check if you're pushing to ecr, so you can make an ecr auth checker
	if opts.AwsAccessKey != "" && opts.AwsSecretKey != "" && opts.AwsRegion != "" && opts.AwsRegistryID != "" {
		return auth.NewAmazonAuth(opts.AwsRegistryID, opts.AwsAccessKey, opts.AwsSecretKey, opts.AwsRegion, opts.AwsStrictAuth), nil
	}

	if opts.AzureClientID != "" && opts.AzureClientSecret != "" && opts.AzureSubscriptionID != "" && opts.AzureTenantID != "" && opts.AzureResourceGroupName != "" && opts.AzureRegistryName != "" && opts.AzureLoginServer != "" {
		return auth.NewAzure(opts.AzureClientID, opts.AzureClientSecret, opts.AzureSubscriptionID, opts.AzureTenantID, opts.AzureResourceGroupName, opts.AzureRegistryName, opts.AzureLoginServer)
	}

	parts := strings.Split(reg, "/")
	apiVersion := parts[len(parts)-2]
	if apiVersion == "v1" {
		registryURL, err := url.Parse(reg)
		if err != nil {
			return nil, err
		}
		return auth.NewDockerAuthV1(registryURL, opts.Username, opts.Password), nil
	} else if apiVersion == "v2" {
		registryURL, err := url.Parse(reg)
		if err != nil {
			return nil, err
		}
		return auth.NewDockerAuth(registryURL, opts.Username, opts.Password), nil
	}
	return nil, ErrNoAuthenticator
}
