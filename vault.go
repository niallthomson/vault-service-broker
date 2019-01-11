package main

import (
	"fmt"
	"html/template"
	"io"
)

const (
	ServicePolicyTemplateWithoutNames string = `
path "cf/{{ .ServiceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ServiceID }}/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .SpaceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .SpaceID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .OrgID }}" {
  capabilities = ["list"]
}

path "cf/{{ .OrgID }}/*" {
  capabilities = ["read", "list"]
}
`

	// ServicePolicyTemplateWithNames is identical to the above, but adds paths for name-ID mount path combos
	ServicePolicyTemplateWithNames string = `
path "cf/{{ .ServiceName }}-{{ .ServiceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ServiceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .ServiceName }}-{{ .ServiceID }}/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .ServiceID }}/*" {
	capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .SpaceName }}-{{ .SpaceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .SpaceID }}" {
  capabilities = ["list"]
}

path "cf/{{ .SpaceName }}-{{ .SpaceID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .SpaceID }}/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}

path "cf/{{ .OrgName }}-{{ .OrgID }}" {
  capabilities = ["list"]
}

path "cf/{{ .OrgID }}" {
  capabilities = ["list"]
}

path "cf/{{ .OrgName }}-{{ .OrgID }}/*" {
  capabilities = ["read", "list"]
}

path "cf/{{ .OrgID }}/*" {
  capabilities = ["read", "list"]
}
`
)

// ServicePolicyTemplateInput is used as input to the ServicePolicyTemplateWithoutNames.
type ServicePolicyTemplateInput struct {
	ServiceName string
	ServiceID   string
	SpaceName   string
	SpaceID     string
	OrgName     string
	OrgID       string
}

// GeneratePolicy takes an io.Writer object and template input and renders the
// resulting template into the writer.
func GeneratePolicy(w io.Writer, i *ServicePolicyTemplateInput) error {
	if i.ServiceName == "" && i.SpaceName == "" && i.OrgName == "" {
		tmpl, err := template.New("service").Parse(ServicePolicyTemplateWithoutNames)
		if err != nil {
			return err
		}
		return tmpl.Execute(w, i)
	}
	if i.ServiceName != "" && i.SpaceName != "" && i.OrgName != "" {
		tmpl, err := template.New("service").Parse(ServicePolicyTemplateWithNames)
		if err != nil {
			return err
		}
		return tmpl.Execute(w, i)
	}
	return fmt.Errorf("all or no object names must be provided, but received %+v", i)
}
