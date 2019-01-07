package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/hashicorp/vault/api"
	"github.com/pivotal-cf/brokerapi"
)

const (
	// REQUIRED CONFIG PARAMETERS
	SecurityUserName = "SECURITY_USER_NAME"
	SecurityUserPassword = "SECURITY_USER_PASSWORD"
	VaultToken = "VAULT_TOKEN"

	// OPTIONAL CONFIG PARAMETERS
	Port = "PORT"
	DefaultPort = ":8000"

	// ServiceID is the UUID of the services
	ServiceID = "SERVICE_ID"
	DefaultServiceID = "0654695e-0760-a1d4-1cad-5dd87b75ed99"

	// VaultAddr is the address of the Vault cluster that this service broker should use
	VaultAddr = "VAULT_ADDR"
	DefaultVaultAddr = "https://127.0.0.1:8200"

	// VaultAdvertiseAddr is the address that OTHERS (users) should use to access Vault
	// It defaults to the VaultAddr if not provided.
	VaultAdvertiseAddr = "VAULT_ADVERTISE_ADDR"

	// ServiceName is the name of the service in the marketplace
	ServiceName = "SERVICE_NAME"
	DefaultServiceName = "hashicorp-vault"

	// ServiceDescription is the service description in the marketplace
	ServiceDescription = "SERVICE_DESCRIPTION"
	DefaultServiceDescription = "HashiCorp Vault Service Broker"

	// PlanName is the name of our plan, only one is supported
	PlanName = "PLAN_NAME"
	DefaultPlanName = "shared"

	// PlanDescription is the plan's description
	PlanDescription = "PLAN_DESCRIPTION"
	DefaultPlanDescription = "Secure access to Vault's storage and transit backends"

	// These are optional; if not provided, none will be added
	ServiceTags = "SERVICE_TAGS"

	// Denotes whether the service broker should automatically renew the service broker's token
	VaultRenew = "VAULT_RENEW"
	DefaultVaultRenew = "true"
)


func main() {
	// Setup the logger - intentionally do not log date or time because it will
	// be prefixed in the log output by CF.
	logger := log.New(os.Stdout, "", 0)

	// Parse required settings
	username := os.Getenv(SecurityUserName)
	if username == "" {
		logger.Fatalf("[ERR] missing %s", SecurityUserName)
	}
	password := os.Getenv(SecurityUserPassword)
	if password == "" {
		logger.Fatalf("[ERR] missing %s", SecurityUserPassword)
	}
	if v := os.Getenv(VaultToken); v == "" {
		logger.Fatalf("[ERR] missing %s", VaultToken)
	}

	// Parse optional settings
	serviceID := getOrDefault(ServiceID, DefaultServiceID)
	serviceName := getOrDefault(ServiceName, DefaultServiceName)
	serviceDescription := getOrDefault(ServiceDescription, DefaultServiceDescription)
	planName := getOrDefault(PlanName, DefaultPlanName)
	planDescription := getOrDefault(PlanDescription, DefaultPlanDescription)
	serviceTags := strings.Split(os.Getenv(ServiceTags), ",")
	vaultAddr := normalizeAddr(getOrDefault(VaultAddr, DefaultVaultAddr))
	vaultAdvertiseAddr := getOrDefault(VaultAdvertiseAddr, vaultAddr)
	port := getOrDefault(Port, DefaultPort)
	if port[0] != ':' {
		port = ":" + port
	}
	renew, err := strconv.ParseBool(getOrDefault(VaultRenew, DefaultVaultRenew))
	if err != nil {
		logger.Fatalf("[ERR] failed to parse %s: %s", VaultRenew, err)
	}

	// Setup the vault client
	client, err := api.NewClient(nil)
	if err != nil {
		logger.Fatal("[ERR] failed to create api client", err)
	}

	// Setup the broker
	broker := &Broker{
		log:    logger,
		client: client,

		serviceID:          serviceID,
		serviceName:        serviceName,
		serviceDescription: serviceDescription,
		serviceTags:        serviceTags,

		planName:        planName,
		planDescription: planDescription,

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

func getOrDefault(settingName, settingDefault string) string {
	if settingValue := os.Getenv(settingName); settingValue != "" {
		return settingValue
	}
	return settingDefault
}
