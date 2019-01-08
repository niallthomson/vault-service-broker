package main

import (
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
	"github.com/pivotal-cf/brokerapi"
)

const (
	// If settings are stored in CredHub, the prefix for all relevant settings.
	// For example: VAULT_SERVICE_BROKER_SECURITY_USER_NAME
	CredhubPrefix = "VAULT_SERVICE_BROKER_"

	// REQUIRED CONFIG PARAMETERS
	SecurityUserName     = "SECURITY_USER_NAME"
	SecurityUserPassword = "SECURITY_USER_PASSWORD"
	VaultToken           = "VAULT_TOKEN"

	// OPTIONAL CONFIG PARAMETERS
	CredhubURL = "CREDHUB_URL"

	Port        = "PORT"
	DefaultPort = ":8000"

	// ServiceID is the UUID of the services
	ServiceID        = "SERVICE_ID"
	DefaultServiceID = "0654695e-0760-a1d4-1cad-5dd87b75ed99"

	// VaultAdvertiseAddr is the address of the Vault cluster that this service broker should use,
	// and that should be used by USERS of the service broker.
	// If it's unset, it'll fall back to the VaultAddr. It's not necessary to have both,
	// but previous code was written this way and they're both retained for backwards compatibility.
	VaultAddr          = "VAULT_ADDR"
	DefaultVaultAddr   = "https://127.0.0.1:8200"
	VaultAdvertiseAddr = "VAULT_ADVERTISE_ADDR"

	// ServiceName is the name of the service in the marketplace
	ServiceName        = "SERVICE_NAME"
	DefaultServiceName = "hashicorp-vault"

	// ServiceDescription is the service description in the marketplace
	ServiceDescription        = "SERVICE_DESCRIPTION"
	DefaultServiceDescription = "HashiCorp Vault Service Broker"

	// PlanName is the name of our plan, only one is supported
	PlanName        = "PLAN_NAME"
	DefaultPlanName = "shared"

	// PlanDescription is the plan's description
	PlanDescription        = "PLAN_DESCRIPTION"
	DefaultPlanDescription = "Secure access to Vault's storage and transit backends"

	// These are optional; if not provided, none will be added
	ServiceTags = "SERVICE_TAGS"

	// Denotes whether the service broker should automatically renew the service broker's token
	VaultRenew        = "VAULT_RENEW"
	DefaultVaultRenew = "true"
)

func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF
	logger := log.New(os.Stdout, "", 0)

	h := newSettingHandler(logger, os.Getenv(CredhubURL))

	// Parse required settings
	username := h.GetOrDefault(SecurityUserName, "")
	if username == "" {
		logger.Fatalf("[ERR] missing %s", SecurityUserName)
	}
	password := h.GetOrDefault(SecurityUserPassword, "")
	if password == "" {
		logger.Fatalf("[ERR] missing %s", SecurityUserPassword)
	}
	if vaultToken := h.GetOrDefault(VaultToken, ""); vaultToken == "" {
		logger.Fatalf("[ERR] missing %s", VaultToken)
	}

	// Parse optional settings
	vaultAddr := normalizeAddr(h.GetOrDefault(VaultAddr, DefaultVaultAddr))
	vaultAdvertiseAddr := h.GetOrDefault(VaultAdvertiseAddr, vaultAddr)
	port := h.GetOrDefault(Port, DefaultPort)
	if port[0] != ':' {
		port = ":" + port
	}
	renew, err := strconv.ParseBool(h.GetOrDefault(VaultRenew, DefaultVaultRenew))
	if err != nil {
		logger.Fatalf("[ERR] failed to parse %s: %s", VaultRenew, err)
	}

	// Setup the vault vaultClient
	client, err := api.NewClient(nil)
	if err != nil {
		logger.Fatal("[ERR] failed to create api vaultClient", err)
	}

	// Setup the broker
	broker := &Broker{
		log:    logger,
		client: client,

		serviceID:          h.GetOrDefault(ServiceID, DefaultServiceID),
		serviceName:        h.GetOrDefault(ServiceName, DefaultServiceName),
		serviceDescription: h.GetOrDefault(ServiceDescription, DefaultServiceDescription),
		serviceTags:        strings.Split(os.Getenv(ServiceTags), ","),

		planName:        h.GetOrDefault(PlanName, DefaultPlanName),
		planDescription: h.GetOrDefault(PlanDescription, DefaultPlanDescription),

		vaultAdvertiseAddr: vaultAdvertiseAddr,
		vaultRenewToken:    renew,
	}
	if err := broker.Start(); err != nil {
		logger.Fatalf("[ERR] failed to start broker: %s", err)
	}

	// Parse the broker credentials
	creds := brokerapi.BrokerCredentials{
		Username: username,
		Password: password,
	}

	// Setup the HTTP handler
	handler := brokerapi.New(broker, lager.NewLogger("vault-broker"), creds)

	// Listen to incoming connection
	serverCh := make(chan struct{}, 1)
	go func() {
		logger.Printf("[INFO] starting server on %s", port)
		if err := http.ListenAndServe(port, handler); err != nil {
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

func newSettingHandler(logger *log.Logger, credhubURL string) *settingHandler {
	// TODO client authorization? Should I assume I don't need to worry about this if they've set up certs correctly?
	if credhubURL == "" {
		return &settingHandler{logger, nil}
	}
	return &settingHandler{logger, credhub.New(credhubURL, cleanhttp.DefaultClient())}
}

type settingHandler struct {
	logger        *log.Logger
	credhubClient *credhub.Client
}

func (h *settingHandler) GetOrDefault(settingName, settingDefault string) string {
	if h.credhubClient != nil {
		latest, err := h.credhubClient.GetLatestByName(CredhubPrefix + settingName)
		if err != nil {
			// "Name Not Found" is returned when the CredHub API returns a 404.
			// It's not ideal that we need to match a string
			// but there's nothing better to check against.
			// In case the error string changes slightly with future SDK versions,
			// we match flexibly.
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				// This variable isn't stored in CredHub, see if it's an env variable.
				goto CheckEnv
			}
			h.logger.Fatalf("error pulling settings from credhub: %s", err)
		}
		settingValue, ok := latest.Value.(string)
		if !ok {
			h.logger.Fatalf("we only support credhub values as string, but received %s as a %s", settingName, reflect.TypeOf(latest.Value))
		}
		if settingValue != "" {
			return settingValue
		}
	}
CheckEnv:
	if settingValue := os.Getenv(settingName); settingValue != "" {
		return settingValue
	}
	return settingDefault
}
