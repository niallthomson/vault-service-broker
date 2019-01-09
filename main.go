package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/go-credhub"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/vault/api"
	"github.com/kelseyhightower/envconfig"
	"github.com/pivotal-cf/brokerapi"
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF
	logger := log.New(os.Stdout, "", 0)

	config, err := parseConfig()
	if err != nil {
		logger.Fatalf("failed to parse configuration: %s", err)
	}

	// Setup the vault vaultClient
	client, err := api.NewClient(nil)
	if err != nil {
		logger.Fatal("[ERR] failed to create api client", err)
	}

	// Setup the broker
	broker := &Broker{
		log:    logger,
		client: client,

		serviceID:          config.ServiceID,
		serviceName:        config.ServiceName,
		serviceDescription: config.ServiceDescription,
		serviceTags:        config.ServiceTags,

		planName:        config.PlanName,
		planDescription: config.PlanDescription,

		vaultAdvertiseAddr: config.VaultAdvertiseAddr,
		vaultRenewToken:    config.VaultRenew,
	}
	if err := broker.Start(); err != nil {
		logger.Fatalf("[ERR] failed to start broker: %s", err)
	}

	// Parse the broker credentials
	creds := brokerapi.BrokerCredentials{
		Username: config.SecurityUserName,
		Password: config.SecurityUserPassword,
	}

	// Setup the HTTP handler
	handler := brokerapi.New(broker, lager.NewLogger("vault-broker"), creds)

	// Listen to incoming connection
	serverCh := make(chan struct{}, 1)
	go func() {
		logger.Printf("[INFO] starting server on %s", config.Port)
		if err := http.ListenAndServe(config.Port, handler); err != nil {
			logger.Fatalf("[ERR] server exited with: %s", err)
		}
		close(serverCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGTERM, syscall.SIGINT)

	select {
	case <-serverCh:
	case s := <-signalCh:
		logger.Printf("[INFO] received signal %s", s)
	}

	if err := broker.Stop(); err != nil {
		logger.Fatalf("[ERR] faild to stop broker: %s", err)
	}

	os.Exit(0)
}

// normalizeAddr takes a string that represents a URL and ensures it has a
// scheme (defaulting to https), and ensures the path ends in a trailing slash.
func normalizeAddr(s string) string {
	if s == "" {
		return s
	}

	u, err := url.Parse(s)
	if err != nil {
		return s
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	if strings.Contains(u.Scheme, ".") {
		u.Host = u.Scheme
		if u.Opaque != "" {
			u.Host = u.Host + ":" + u.Opaque
			u.Opaque = ""
		}
		u.Scheme = "https"
	}

	if u.Host == "" {
		split := strings.SplitN(u.Path, "/", 2)
		switch len(split) {
		case 0:
		case 1:
			u.Host = split[0]
			u.Path = "/"
		case 2:
			u.Host = split[0]
			u.Path = split[1]
		}
	}

	u.Path = strings.TrimRight(u.Path, "/") + "/"

	return u.String()
}

// parseConfig is broken into its own function for testability
func parseConfig() (*Configuration, error) {
	config := &Configuration{}
	if err := envconfig.Process("", config); err != nil {
		return nil, err
	}
	if config.CredhubURL != "" {
		if err := credhubProcess("VAULT_SERVICE_BROKER_", config); err != nil {
			return nil, err
		}
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

type Configuration struct {
	// Required
	SecurityUserName     string `envconfig:"security_user_name"`
	SecurityUserPassword string `envconfig:"security_user_password"`
	VaultToken           string `envconfig:"vault_token"`

	// Optional
	CredhubURL         string   `envconfig:"credhub_url"`
	Port               string   `envconfig:"port" default:":8000"`
	ServiceID          string   `envconfig:"service_id" default:"0654695e-0760-a1d4-1cad-5dd87b75ed99"`
	VaultAddr          string   `envconfig:"vault_addr" default:"https://127.0.0.1:8200"`
	VaultAdvertiseAddr string   `envconfig:"vault_advertise_addr"`
	ServiceName        string   `envconfig:"service_name" default:"hashicorp-vault"`
	ServiceDescription string   `envconfig:"service_description" default:"HashiCorp Vault Service Broker"`
	PlanName           string   `envconfig:"plan_name" default:"shared"`
	PlanDescription    string   `envconfig:"plan_description" default:"Secure access to Vault's storage and transit backends"`
	ServiceTags        []string `envconfig:"service_tags"`
	VaultRenew         bool     `envconfig:"vault_renew" default:"true"`
}

func (c *Configuration) Validate() error {
	// Ensure required parameters were provided
	if c.SecurityUserName == "" {
		return errors.New("missing SECURITY_USER_NAME")
	}
	if c.SecurityUserPassword == "" {
		return errors.New("missing SECURITY_USER_PASSWORD")
	}
	if c.VaultToken == "" {
		return errors.New("missing VAULT_TOKEN")
	}

	// If these values aren't perfect, we can fix them
	if c.Port[0] != ':' {
		c.Port = ":" + c.Port
	}
	if c.VaultAdvertiseAddr == "" {
		c.VaultAdvertiseAddr = c.VaultAddr
	}
	c.VaultAddr = normalizeAddr(c.VaultAddr)
	c.VaultAdvertiseAddr = normalizeAddr(c.VaultAdvertiseAddr)
	return nil
}

func credhubProcess(prefix string, config *Configuration) error {

	client := credhub.New(config.CredhubURL, cleanhttp.DefaultClient())

	// Pull the "envconfig" field name from each field and look for it in Credhub
	configTypeInfo := reflect.TypeOf(*config)
	settableConfig := reflect.ValueOf(config).Elem()

	for i := 0; i < configTypeInfo.NumField(); i++ {
		fieldTypeInfo := configTypeInfo.Field(i)
		credhubName := prefix + strings.ToUpper(fieldTypeInfo.Tag.Get("envconfig"))

		latest, err := client.GetLatestByName(credhubName)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			return err
		}
		if latest == nil {
			// This key doesn't exist in Credhub
			continue
		}
		settingValue, ok := latest.Value.(string)
		if !ok {
			return fmt.Errorf("we only support credhub values as bash-like string values, but received %s as a %s", credhubName, reflect.TypeOf(latest.Value))
		}
		if settingValue == "" {
			// The value for this key isn't set in Credhub
			continue
		}

		// Update the value for this field with Credhub's value
		settableField := settableConfig.Field(i)
		switch fieldTypeInfo.Type.Kind() {
		case reflect.Bool:
			asBool, err := strconv.ParseBool(settingValue)
			if err != nil {
				return fmt.Errorf("error parsing bool %s: %s", credhubName, err)
			}
			settableField.SetBool(asBool)
		case reflect.String:
			settableField.SetString(settingValue)
		case reflect.Slice:
			settableField.Set(reflect.ValueOf(strings.Split(settingValue, ",")))
		default:
			return fmt.Errorf("unsupported type of %s for %s", fieldTypeInfo.Type.Kind(), credhubName)
		}
	}
	return nil
}
